package analyze

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type WebPresenter struct {
	addr      string
	baseURL   string
	httpSrv   *http.Server
	readyOnce sync.Once
	readyCh   chan string
}

func NewWebPresenter() *WebPresenter {
	return &WebPresenter{addr: "127.0.0.1:0", readyCh: make(chan string, 1)}
}

func (p *WebPresenter) Ready() <-chan string {
	return p.readyCh
}

func (p *WebPresenter) Run(ctx context.Context, state *StateStore, worker func(context.Context) error) error {
	listener, err := net.Listen("tcp", p.addr)
	if err != nil {
		return fmt.Errorf("starting analysis web listener: %w", err)
	}
	defer listener.Close()

	p.baseURL = "http://" + listener.Addr().String()
	p.publishReady(p.baseURL)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(webPage))
	})
	mux.HandleFunc("/snapshot", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(state.Snapshot())
	})
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
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
			case <-ctx.Done():
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

	p.httpSrv = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	serveErrCh := make(chan error, 1)
	go func() {
		serveErr := p.httpSrv.Serve(listener)
		if serveErr != nil && serveErr != http.ErrServerClosed {
			serveErrCh <- serveErr
			return
		}
		serveErrCh <- nil
	}()

	workerErrCh := make(chan error, 1)
	go func() {
		err := worker(ctx)
		if err == nil {
			state.Update(markSessionComplete)
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = p.httpSrv.Shutdown(shutdownCtx)
		workerErrCh <- err
	}()

	serveErr := <-serveErrCh
	workerErr := <-workerErrCh
	if serveErr != nil {
		return serveErr
	}
	if workerErr == context.Canceled {
		return nil
	}
	return workerErr
}

func (p *WebPresenter) publishReady(url string) {
	p.readyOnce.Do(func() {
		p.readyCh <- url
	})
}

const webPage = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>thresher analyze</title>
  <style>
    :root { color-scheme: dark; }
    body { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; margin: 0; background: #07111d; color: #e5eef7; }
    main { max-width: 1100px; margin: 0 auto; padding: 24px; }
    h1 { margin: 0 0 16px; font-size: 28px; }
    .grid { display: grid; gap: 16px; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); }
    .card { background: #102235; border: 1px solid #1e3a56; border-radius: 12px; padding: 16px; }
    .label { color: #8fb7d8; font-size: 12px; text-transform: uppercase; letter-spacing: 0.08em; }
    .value { margin-top: 8px; font-size: 18px; white-space: pre-wrap; word-break: break-word; }
    pre { margin: 0; white-space: pre-wrap; word-break: break-word; }
    ul { margin: 8px 0 0; padding-left: 18px; }
  </style>
</head>
<body>
  <main>
    <h1>thresher analyze</h1>
    <div class="grid">
      <section class="card"><div class="label">Status</div><div id="status" class="value">loading</div></section>
      <section class="card"><div class="label">Phase</div><div id="phase" class="value">loading</div></section>
      <section class="card"><div class="label">Model</div><div id="model" class="value">loading</div></section>
      <section class="card"><div class="label">Counters</div><div id="counters" class="value">loading</div></section>
    </div>
    <div class="grid" style="margin-top:16px;">
      <section class="card"><div class="label">Analysis</div><pre id="analysis" class="value"></pre></section>
      <section class="card"><div class="label">Events</div><ul id="events" class="value"></ul></section>
    </div>
  </main>
  <script>
    const statusEl = document.getElementById('status');
    const phaseEl = document.getElementById('phase');
    const modelEl = document.getElementById('model');
    const countersEl = document.getElementById('counters');
    const analysisEl = document.getElementById('analysis');
    const eventsEl = document.getElementById('events');

    function render(snapshot) {
      statusEl.textContent = snapshot.status || 'waiting';
      phaseEl.textContent = snapshot.phase || 'idle';
      modelEl.textContent = snapshot.model || 'unknown';
      countersEl.textContent = [
        'packets: ' + snapshot.records,
        'bytes: ' + snapshot.total_bytes,
        'pending: ' + snapshot.pending_packets + ' / ' + snapshot.pending_bytes,
        'uploaded batches: ' + snapshot.uploaded_batches,
        'paused: ' + snapshot.paused,
        'limit reached: ' + snapshot.limit_reached,
        'completed: ' + snapshot.completed
      ].join('\n');
      analysisEl.textContent = (snapshot.analysis || []).join('\n\n') || 'Waiting for Aperture analysis...';
      eventsEl.innerHTML = '';
      (snapshot.events || []).forEach((event) => {
        const li = document.createElement('li');
        li.textContent = event;
        eventsEl.appendChild(li);
      });
    }

    fetch('/snapshot').then((resp) => resp.json()).then(render);
    const stream = new EventSource('/events');
    stream.addEventListener('snapshot', (event) => render(JSON.parse(event.data)));
  </script>
</body>
</html>`

func IsLocalhostURL(raw string) bool {
	if !strings.HasPrefix(raw, "http://") {
		return false
	}
	hostPort := strings.TrimPrefix(raw, "http://")
	host, _, err := net.SplitHostPort(hostPort)
	if err != nil {
		return false
	}
	return host == "127.0.0.1" || host == "localhost"
}
