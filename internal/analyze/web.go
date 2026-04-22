package analyze

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	neturl "net/url"
	"strings"
	"sync"
	"time"
)

type WebPresenter struct {
	runtime   webRuntime
	baseURL   string
	httpSrv   *http.Server
	readyOnce sync.Once
	readyCh   chan string
}

type webModelRequest struct {
	Model string `json:"model"`
}

type webPauseRequest struct {
	Paused *bool `json:"paused,omitempty"`
}

func NewWebPresenter(config Config) *WebPresenter {
	return newWebPresenterWithRuntime(newWebRuntime(config))
}

func newWebPresenterWithRuntime(runtime webRuntime) *WebPresenter {
	return &WebPresenter{runtime: runtime, readyCh: make(chan string, 1)}
}

func (p *WebPresenter) Ready() <-chan string {
	return p.readyCh
}

func (p *WebPresenter) Run(ctx context.Context, state *StateStore, worker func(context.Context) error) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	endpoint, err := p.runtime.Open(runCtx)
	if err != nil {
		return err
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = endpoint.shutdown(shutdownCtx)
	}()

	p.baseURL = endpoint.baseURL
	p.publishReady(p.baseURL)

	handler := p.routes(runCtx, cancel, state, endpoint.routePrefix)
	if endpoint.wrap != nil {
		handler = endpoint.wrap(handler)
	}

	p.httpSrv = &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	serveErrCh := make(chan error, 1)
	go func() {
		serveErr := p.httpSrv.Serve(endpoint.listener)
		if serveErr != nil && serveErr != http.ErrServerClosed {
			serveErrCh <- serveErr
			return
		}
		serveErrCh <- nil
	}()

	workerErrCh := make(chan error, 1)
	go func() {
		err := worker(runCtx)
		if err == nil && runCtx.Err() == nil {
			state.Update(markSessionComplete)
		}
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = p.httpSrv.Shutdown(shutdownCtx)
		workerErrCh <- err
	}()

	serveErr := <-serveErrCh
	workerErr := <-workerErrCh
	if serveErr != nil {
		return serveErr
	}
	if errors.Is(workerErr, context.Canceled) || errors.Is(runCtx.Err(), context.Canceled) {
		return nil
	}
	return workerErr
}

func (p *WebPresenter) routes(runCtx context.Context, cancel context.CancelFunc, state *StateStore, routePrefix string) http.Handler {
	app := http.NewServeMux()
	app.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(webPage))
	})
	app.HandleFunc("/snapshot", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, state.Snapshot())
	})
	app.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		snapshots, unsubscribe := state.Subscribe(32)
		defer unsubscribe()

		for {
			select {
			case <-r.Context().Done():
				return
			case <-runCtx.Done():
				return
			case snapshot, ok := <-snapshots:
				if !ok {
					return
				}
				payload, err := json.Marshal(snapshot)
				if err != nil {
					return
				}
				_, _ = fmt.Fprintf(w, "event: snapshot\ndata: %s\n\n", payload)
				flusher.Flush()
				if snapshot.Completed || snapshot.Error != "" {
					return
				}
			}
		}
	})
	app.HandleFunc("/control/model", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req webModelRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		model := strings.TrimSpace(req.Model)
		if model == "" {
			http.Error(w, "model is required", http.StatusBadRequest)
			return
		}

		snapshot := state.Snapshot()
		if len(snapshot.Models) > 0 && !containsString(snapshot.Models, model) {
			http.Error(w, "unknown model", http.StatusBadRequest)
			return
		}

		state.SetActiveModel(model)
		writeJSON(w, state.Snapshot())
	})
	app.HandleFunc("/control/pause", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		paused := !state.IsPaused()
		if r.Body != nil && r.ContentLength != 0 {
			var req webPauseRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "invalid request body", http.StatusBadRequest)
				return
			}
			if req.Paused != nil {
				paused = *req.Paused
			}
		}

		state.SetPaused(paused)
		writeJSON(w, state.Snapshot())
	})
	app.HandleFunc("/control/quit", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		state.Update(func(snapshot *SessionSnapshot) {
			snapshot.Status = "quit requested"
			snapshot.LastEvent = "quit requested from web UI"
			snapshot.Phase = "complete"
			snapshot.Completed = true
			pushSnapshotEvent(snapshot, snapshot.LastEvent)
		})
		writeJSON(w, map[string]bool{"ok": true})
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		go cancel()
	})

	_ = routePrefix
	return app
}

func (p *WebPresenter) publishReady(url string) {
	p.readyOnce.Do(func() {
		p.readyCh <- url
	})
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

const webPage = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>thresher analyze</title>
  <style>
    :root {
      color-scheme: dark;
      --bg: #081019;
      --bg-accent: #0f2234;
      --panel: rgba(11, 24, 38, 0.88);
      --panel-strong: rgba(16, 35, 56, 0.96);
      --border: rgba(120, 181, 214, 0.22);
      --text: #eef6ff;
      --muted: #9cb3c6;
      --subtle: #7e96aa;
      --accent: #75d0bf;
      --accent-strong: #d8f171;
      --warn: #ffc36b;
      --danger: #ff7f7d;
      --shadow: 0 24px 80px rgba(2, 8, 16, 0.45);
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      min-height: 100vh;
      font-family: "Avenir Next", "Segoe UI Variable", "Helvetica Neue", sans-serif;
      background:
        radial-gradient(circle at top left, rgba(117, 208, 191, 0.15), transparent 28%),
        radial-gradient(circle at top right, rgba(216, 241, 113, 0.09), transparent 24%),
        linear-gradient(180deg, var(--bg-accent), var(--bg));
      color: var(--text);
    }
    .shell {
      width: min(1440px, calc(100vw - 40px));
      margin: 0 auto;
      padding: 28px 0 36px;
    }
    .masthead {
      display: flex;
      justify-content: space-between;
      gap: 24px;
      align-items: flex-end;
      margin-bottom: 24px;
    }
    .brand {
      display: flex;
      flex-direction: column;
      gap: 6px;
    }
    .eyebrow {
      color: var(--accent-strong);
      text-transform: uppercase;
      letter-spacing: 0.18em;
      font-size: 11px;
      font-weight: 700;
    }
    h1 {
      margin: 0;
      font-size: clamp(28px, 4vw, 46px);
      letter-spacing: -0.04em;
      line-height: 0.95;
    }
    .subtitle {
      color: var(--muted);
      max-width: 760px;
      font-size: 15px;
    }
    .hero-meta {
      display: flex;
      flex-wrap: wrap;
      gap: 10px;
      justify-content: flex-end;
    }
    .pill {
      border: 1px solid var(--border);
      background: rgba(255, 255, 255, 0.04);
      border-radius: 999px;
      padding: 10px 14px;
      font-size: 13px;
      color: var(--muted);
      backdrop-filter: blur(10px);
    }
    .layout {
      display: grid;
      grid-template-columns: minmax(0, 1.8fr) minmax(320px, 0.95fr);
      gap: 18px;
      align-items: start;
    }
    .stack {
      display: grid;
      gap: 18px;
    }
    .panel {
      background: var(--panel);
      border: 1px solid var(--border);
      border-radius: 22px;
      box-shadow: var(--shadow);
      overflow: hidden;
      backdrop-filter: blur(14px);
    }
    .panel-header {
      display: flex;
      justify-content: space-between;
      gap: 16px;
      align-items: center;
      padding: 18px 20px 12px;
    }
    .panel-title {
      margin: 0;
      font-size: 13px;
      letter-spacing: 0.16em;
      text-transform: uppercase;
      color: var(--subtle);
    }
    .panel-body {
      padding: 0 20px 20px;
    }
    .stats {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 12px;
    }
    .stat {
      padding: 14px 16px;
      border-radius: 16px;
      background: rgba(255, 255, 255, 0.03);
      border: 1px solid rgba(255, 255, 255, 0.05);
    }
    .stat-label {
      color: var(--subtle);
      text-transform: uppercase;
      letter-spacing: 0.12em;
      font-size: 11px;
      margin-bottom: 8px;
    }
    .stat-value {
      font-size: 24px;
      font-weight: 700;
      letter-spacing: -0.04em;
    }
    .stat-meta {
      margin-top: 6px;
      color: var(--muted);
      font-size: 13px;
    }
    .controls {
      display: grid;
      gap: 12px;
    }
    .control-row {
      display: grid;
      gap: 10px;
    }
    label {
      color: var(--subtle);
      font-size: 12px;
      text-transform: uppercase;
      letter-spacing: 0.13em;
    }
    select, button {
      width: 100%;
      border: 1px solid var(--border);
      border-radius: 14px;
      padding: 12px 14px;
      font: inherit;
      color: var(--text);
      background: var(--panel-strong);
    }
    select:disabled, button:disabled { opacity: 0.55; cursor: not-allowed; }
    .button-row {
      display: grid;
      gap: 10px;
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }
    .button-accent {
      background: linear-gradient(135deg, rgba(117, 208, 191, 0.24), rgba(117, 208, 191, 0.08));
      border-color: rgba(117, 208, 191, 0.42);
    }
    .button-warn {
      background: linear-gradient(135deg, rgba(255, 195, 107, 0.2), rgba(255, 195, 107, 0.08));
      border-color: rgba(255, 195, 107, 0.4);
    }
    .button-danger {
      background: linear-gradient(135deg, rgba(255, 127, 125, 0.24), rgba(255, 127, 125, 0.08));
      border-color: rgba(255, 127, 125, 0.45);
    }
    .keyline {
      display: grid;
      gap: 10px;
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }
    .keyline-item {
      padding: 14px 16px;
      border-radius: 16px;
      background: rgba(255, 255, 255, 0.03);
      border: 1px solid rgba(255, 255, 255, 0.05);
    }
    .keyline-item strong {
      display: block;
      margin-top: 6px;
      font-size: 18px;
      letter-spacing: -0.03em;
    }
    .mono {
      font-family: "IBM Plex Mono", "SFMono-Regular", Menlo, monospace;
    }
    .analysis-list {
      display: grid;
      gap: 12px;
      max-height: 62vh;
      overflow: auto;
      padding-right: 4px;
    }
    .analysis-entry {
      border-radius: 18px;
      padding: 16px 18px;
      background: rgba(255, 255, 255, 0.025);
      border: 1px solid rgba(255, 255, 255, 0.06);
    }
    .analysis-entry h3 {
      margin: 0 0 10px;
      font-size: 12px;
      letter-spacing: 0.14em;
      text-transform: uppercase;
      color: var(--accent);
    }
    .analysis-entry pre {
      margin: 0;
      white-space: pre-wrap;
      word-break: break-word;
      color: #edf3fa;
      line-height: 1.48;
      font-family: "IBM Plex Mono", "SFMono-Regular", Menlo, monospace;
      font-size: 13px;
    }
    .placeholder {
      color: var(--muted);
      font-size: 15px;
      padding: 12px 4px 2px;
    }
    .event-list {
      display: grid;
      gap: 10px;
      max-height: 34vh;
      overflow: auto;
      padding-right: 4px;
    }
    .event-item {
      padding: 12px 14px;
      border-radius: 14px;
      background: rgba(255, 255, 255, 0.03);
      border: 1px solid rgba(255, 255, 255, 0.05);
      color: var(--muted);
      font-family: "IBM Plex Mono", "SFMono-Regular", Menlo, monospace;
      font-size: 12px;
      line-height: 1.45;
    }
    .status-bar {
      display: flex;
      flex-wrap: wrap;
      gap: 10px;
    }
    .status-chip {
      padding: 8px 12px;
      border-radius: 999px;
      font-size: 12px;
      font-weight: 700;
      letter-spacing: 0.08em;
      text-transform: uppercase;
      border: 1px solid transparent;
    }
    .status-chip.collecting, .status-chip.idle { background: rgba(117, 208, 191, 0.08); color: var(--accent); border-color: rgba(117, 208, 191, 0.22); }
    .status-chip.uploading { background: rgba(216, 241, 113, 0.12); color: var(--accent-strong); border-color: rgba(216, 241, 113, 0.22); }
    .status-chip.paused, .status-chip.limited { background: rgba(255, 195, 107, 0.14); color: var(--warn); border-color: rgba(255, 195, 107, 0.24); }
    .status-chip.error { background: rgba(255, 127, 125, 0.14); color: var(--danger); border-color: rgba(255, 127, 125, 0.24); }
    .status-chip.complete { background: rgba(156, 179, 198, 0.12); color: #dbe8f3; border-color: rgba(156, 179, 198, 0.2); }
    @media (max-width: 1040px) {
      .layout { grid-template-columns: 1fr; }
      .shell { width: min(100vw - 24px, 1440px); padding-top: 18px; }
      .masthead { flex-direction: column; align-items: flex-start; }
      .hero-meta { justify-content: flex-start; }
    }
    @media (max-width: 720px) {
      .stats, .button-row, .keyline { grid-template-columns: 1fr; }
      .panel-header { flex-direction: column; align-items: flex-start; }
    }
  </style>
</head>
<body>
  <main class="shell">
    <header class="masthead">
      <div class="brand">
        <div class="eyebrow">Browser Analysis Session</div>
        <h1>thresher analyze</h1>
        <div class="subtitle">Live packet analysis in web mode, with the same model, pause, and session controls as the console workflow.</div>
      </div>
      <div class="hero-meta">
        <div class="pill mono" id="endpointPill">endpoint http://ai</div>
        <div class="pill mono" id="modelPill">model loading</div>
        <div class="pill mono" id="statusPill">status loading</div>
      </div>
    </header>

    <section class="panel" style="margin-bottom:18px;">
      <div class="panel-header">
        <h2 class="panel-title">Session</h2>
        <div class="status-bar" id="statusBar"></div>
      </div>
      <div class="panel-body stats">
        <div class="stat">
          <div class="stat-label">Packets</div>
          <div class="stat-value mono" id="packetsValue">0</div>
          <div class="stat-meta" id="packetsMeta">decoded records processed</div>
        </div>
        <div class="stat">
          <div class="stat-label">Upload</div>
          <div class="stat-value mono" id="uploadValue">0</div>
          <div class="stat-meta" id="uploadMeta">batches completed</div>
        </div>
        <div class="stat">
          <div class="stat-label">Pending Batch</div>
          <div class="stat-value mono" id="pendingValue">0 / 0</div>
          <div class="stat-meta" id="pendingMeta">packets / bytes waiting</div>
        </div>
        <div class="stat">
          <div class="stat-label">Session Limits</div>
          <div class="stat-value mono" id="limitValue">0 / 0</div>
          <div class="stat-meta" id="limitMeta">packet / byte cap</div>
        </div>
      </div>
    </section>

    <div class="layout">
      <section class="stack">
        <section class="panel">
          <div class="panel-header">
            <h2 class="panel-title">Live Analysis</h2>
            <div class="pill mono" id="analysisState">awaiting updates</div>
          </div>
          <div class="panel-body">
            <div class="analysis-list" id="analysisList"></div>
          </div>
        </section>
      </section>

      <aside class="stack">
        <section class="panel">
          <div class="panel-header">
            <h2 class="panel-title">Controls</h2>
          </div>
          <div class="panel-body controls">
            <div class="control-row">
              <label for="modelSelect">Active model</label>
              <select id="modelSelect"></select>
            </div>
            <div class="button-row">
              <button id="applyModelButton" class="button-accent">Apply model</button>
              <button id="pauseButton" class="button-warn">Pause analysis</button>
            </div>
            <button id="quitButton" class="button-danger">Quit session</button>
          </div>
        </section>

        <section class="panel">
          <div class="panel-header">
            <h2 class="panel-title">State</h2>
          </div>
          <div class="panel-body keyline">
            <div class="keyline-item">
              Last event
              <strong class="mono" id="eventValue">session created</strong>
            </div>
            <div class="keyline-item">
              Batch ceiling
              <strong class="mono" id="batchValue">0 / 0</strong>
            </div>
            <div class="keyline-item">
              Upload bytes
              <strong class="mono" id="bytesValue">0</strong>
            </div>
            <div class="keyline-item">
              In flight
              <strong class="mono" id="flightValue">false</strong>
            </div>
          </div>
        </section>

        <section class="panel">
          <div class="panel-header">
            <h2 class="panel-title">Recent Events</h2>
          </div>
          <div class="panel-body">
            <div class="event-list" id="eventsList"></div>
          </div>
        </section>
      </aside>
    </div>
  </main>

  <script>
    const endpointPill = document.getElementById('endpointPill');
    const modelPill = document.getElementById('modelPill');
    const statusPill = document.getElementById('statusPill');
    const statusBar = document.getElementById('statusBar');
    const packetsValue = document.getElementById('packetsValue');
    const packetsMeta = document.getElementById('packetsMeta');
    const uploadValue = document.getElementById('uploadValue');
    const uploadMeta = document.getElementById('uploadMeta');
    const pendingValue = document.getElementById('pendingValue');
    const pendingMeta = document.getElementById('pendingMeta');
    const limitValue = document.getElementById('limitValue');
    const limitMeta = document.getElementById('limitMeta');
    const analysisState = document.getElementById('analysisState');
    const analysisList = document.getElementById('analysisList');
    const modelSelect = document.getElementById('modelSelect');
    const applyModelButton = document.getElementById('applyModelButton');
    const pauseButton = document.getElementById('pauseButton');
    const quitButton = document.getElementById('quitButton');
    const eventValue = document.getElementById('eventValue');
    const batchValue = document.getElementById('batchValue');
    const bytesValue = document.getElementById('bytesValue');
    const flightValue = document.getElementById('flightValue');
    const eventsList = document.getElementById('eventsList');

    let currentSnapshot = null;

    async function postJSON(path, payload) {
      const response = await fetch(path, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload || {})
      });
      if (!response.ok) {
        const text = await response.text();
        throw new Error(text || 'request failed');
      }
      return response.json().catch(() => null);
    }

    function chip(label, kind) {
      const span = document.createElement('span');
      span.className = 'status-chip ' + kind;
      span.textContent = label;
      return span;
    }

    function disableControls(disabled) {
      modelSelect.disabled = disabled || modelSelect.options.length === 0;
      applyModelButton.disabled = disabled || modelSelect.options.length === 0;
      pauseButton.disabled = disabled;
      quitButton.disabled = disabled;
    }

    function renderModels(snapshot) {
      const selectedValue = modelSelect.value || snapshot.model || '';
      modelSelect.innerHTML = '';
      const models = snapshot.models || [];
      if (models.length === 0) {
        const option = document.createElement('option');
        option.value = '';
        option.textContent = 'model discovery unavailable';
        modelSelect.appendChild(option);
      } else {
        models.forEach((model) => {
          const option = document.createElement('option');
          option.value = model;
          option.textContent = model;
          if (model === selectedValue || (!selectedValue && model === snapshot.model)) {
            option.selected = true;
          }
          modelSelect.appendChild(option);
        });
      }
    }

    function renderAnalysis(snapshot) {
      analysisList.innerHTML = '';
      const items = snapshot.analysis || [];
      if (items.length === 0) {
        const placeholder = document.createElement('div');
        placeholder.className = 'placeholder';
        placeholder.textContent = 'Waiting for Aperture analysis...';
        analysisList.appendChild(placeholder);
        return;
      }

      items.forEach((entry, index) => {
        const article = document.createElement('article');
        article.className = 'analysis-entry';
        const title = document.createElement('h3');
        title.textContent = 'Update ' + (index + 1);
        const pre = document.createElement('pre');
        pre.textContent = entry;
        article.appendChild(title);
        article.appendChild(pre);
        analysisList.appendChild(article);
      });
    }

    function renderEvents(snapshot) {
      eventsList.innerHTML = '';
      const events = (snapshot.events || []).slice().reverse();
      if (events.length === 0) {
        const placeholder = document.createElement('div');
        placeholder.className = 'placeholder';
        placeholder.textContent = 'No session events yet.';
        eventsList.appendChild(placeholder);
        return;
      }

      events.forEach((event) => {
        const item = document.createElement('div');
        item.className = 'event-item';
        item.textContent = event;
        eventsList.appendChild(item);
      });
    }

    function render(snapshot) {
      currentSnapshot = snapshot;

      endpointPill.textContent = 'endpoint ' + (snapshot.endpoint || 'http://ai');
      modelPill.textContent = 'model ' + (snapshot.model || 'unknown');
      statusPill.textContent = 'status ' + (snapshot.status || 'waiting');
      analysisState.textContent = snapshot.error ? 'error' : (snapshot.phase || 'idle');

      statusBar.innerHTML = '';
      statusBar.appendChild(chip(snapshot.phase || 'idle', snapshot.phase || 'idle'));
      if (snapshot.paused) statusBar.appendChild(chip('paused', 'paused'));
      if (snapshot.in_flight) statusBar.appendChild(chip('uploading', 'uploading'));
      if (snapshot.limit_reached) statusBar.appendChild(chip('limit reached', 'limited'));
      if (snapshot.completed) statusBar.appendChild(chip('complete', 'complete'));
      if (snapshot.error) statusBar.appendChild(chip('error', 'error'));

      packetsValue.textContent = String(snapshot.records || 0);
      packetsMeta.textContent = 'decoded records processed';
      uploadValue.textContent = String(snapshot.uploaded_batches || 0);
      uploadMeta.textContent = (snapshot.paused ? 'paused' : 'batches completed');
      pendingValue.textContent = (snapshot.pending_packets || 0) + ' / ' + (snapshot.pending_bytes || 0);
      pendingMeta.textContent = 'packets / bytes waiting';
      limitValue.textContent = (snapshot.session_packets || 0) + ' / ' + (snapshot.session_bytes || 0);
      limitMeta.textContent = 'packet / byte cap';
      eventValue.textContent = snapshot.last_event || 'session created';
      batchValue.textContent = (snapshot.batch_packets || 0) + ' / ' + (snapshot.batch_bytes || 0);
      bytesValue.textContent = String(snapshot.total_bytes || 0);
      flightValue.textContent = String(Boolean(snapshot.in_flight));

      pauseButton.textContent = snapshot.paused ? 'Resume analysis' : 'Pause analysis';
      renderModels(snapshot);
      renderAnalysis(snapshot);
      renderEvents(snapshot);
      disableControls(Boolean(snapshot.completed || snapshot.error));
    }

    applyModelButton.addEventListener('click', async () => {
      if (!modelSelect.value) return;
      try {
        const snapshot = await postJSON('control/model', { model: modelSelect.value });
        if (snapshot) render(snapshot);
      } catch (error) {
        console.error(error);
      }
    });

    pauseButton.addEventListener('click', async () => {
      if (!currentSnapshot) return;
      try {
        const snapshot = await postJSON('control/pause', { paused: !currentSnapshot.paused });
        if (snapshot) render(snapshot);
      } catch (error) {
        console.error(error);
      }
    });

    quitButton.addEventListener('click', async () => {
      quitButton.disabled = true;
      try {
        await postJSON('control/quit', {});
      } catch (error) {
        console.error(error);
      }
    });

    fetch('snapshot')
      .then((response) => response.json())
      .then(render)
      .catch((error) => console.error(error));

    const stream = new EventSource('events');
    stream.addEventListener('snapshot', (event) => render(JSON.parse(event.data)));
    stream.addEventListener('error', () => {
      if (currentSnapshot && (currentSnapshot.completed || currentSnapshot.error)) {
        stream.close();
      }
    });
  </script>
</body>
</html>`

func IsLocalhostURL(raw string) bool {
	parsed, err := neturl.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	if parsed.Scheme != "http" {
		return false
	}
	host := parsed.Hostname()
	if host == "" {
		return false
	}
	return host == "127.0.0.1" || host == "localhost"
}
