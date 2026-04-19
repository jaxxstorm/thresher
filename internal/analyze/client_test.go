package analyze

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jaxxstorm/thresher/internal/capture"
)

func TestClientAnalyzeChatCompletions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"analysis text"}}]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, EndpointChatCompletions)
	resp, err := client.Analyze(context.Background(), AnalyzeRequest{Model: "gpt-4o", Prompt: "analyze"})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if resp.Text != "analysis text" {
		t.Fatalf("unexpected response %#v", resp)
	}
}

func TestClientListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o"},{"id":"claude-haiku"}]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, EndpointAuto)
	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	if len(models) != 2 || models[0].ID != "gpt-4o" {
		t.Fatalf("unexpected models %#v", models)
	}
}

func TestSessionRunReaderBatchesInput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o"}]}`))
		case "/v1/chat/completions":
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"live analysis"}}]}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	session := NewSession(Config{Endpoint: server.URL, Model: "gpt-4o", BatchPackets: 1})
	session.programOpts = []tea.ProgramOption{tea.WithInput(nil), tea.WithOutput(io.Discard)}
	input := bytes.NewBufferString("{\"number\":1,\"time\":\"2026-04-18T00:00:00Z\",\"src\":\"100.64.0.1\",\"dst\":\"100.64.0.2\",\"protocol\":\"TCP\",\"length\":64,\"info\":\"1234 -> 443\"}\n")
	if err := session.RunReader(context.Background(), input); err != nil {
		t.Fatalf("RunReader() error = %v", err)
	}

	snapshot := waitForSnapshot(t, session.State(), func(state SessionSnapshot) bool {
		return state.UploadedBatches == 1 && len(state.Analysis) == 1
	})
	if snapshot.Records != 1 {
		t.Fatalf("expected one processed record, got %#v", snapshot)
	}
	if snapshot.Model != "gpt-4o" {
		t.Fatalf("expected active model to remain gpt-4o, got %#v", snapshot)
	}
	if len(snapshot.Models) != 1 || snapshot.Models[0] != "gpt-4o" {
		t.Fatalf("expected discovered models to be rendered, got %#v", snapshot.Models)
	}
}

func TestSessionRunReaderMarksLimitReached(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o"}]}`))
		case "/v1/chat/completions":
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"live analysis"}}]}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	session := NewSession(Config{Endpoint: server.URL, Model: "gpt-4o", BatchPackets: 10, SessionPackets: 1})
	session.programOpts = []tea.ProgramOption{tea.WithInput(nil), tea.WithOutput(io.Discard)}
	input := bytes.NewBufferString(
		"{\"number\":1,\"time\":\"2026-04-18T00:00:00Z\",\"src\":\"100.64.0.1\",\"dst\":\"100.64.0.2\",\"protocol\":\"TCP\",\"length\":64,\"info\":\"1234 -> 443\"}\n" +
			"{\"number\":2,\"time\":\"2026-04-18T00:00:01Z\",\"src\":\"100.64.0.2\",\"dst\":\"100.64.0.1\",\"protocol\":\"TCP\",\"length\":64,\"info\":\"443 -> 1234\"}\n",
	)
	if err := session.RunReader(context.Background(), input); err != nil {
		t.Fatalf("RunReader() error = %v", err)
	}

	snapshot := waitForSnapshot(t, session.State(), func(state SessionSnapshot) bool {
		return state.LimitReached
	})
	if snapshot.Records != 2 {
		t.Fatalf("expected both records to be observed before the limit stop, got %#v", snapshot)
	}
	if !snapshot.LimitReached {
		t.Fatalf("expected limit reached snapshot, got %#v", snapshot)
	}
}

func TestBuildBatchPromptIncludesAnnotations(t *testing.T) {
	prompt := buildBatchPrompt([]capture.Record{{
		Number:   1,
		Time:     "2026-04-18T00:00:00Z",
		Src:      "100.64.0.1",
		Dst:      "100.64.0.2",
		Protocol: "TCP",
		Info:     "1234 -> 443",
		StreamID: "flow-1",
		Analysis: &capture.Analysis{Annotations: []string{"retransmission"}},
	}})
	if prompt == "" || !bytes.Contains([]byte(prompt), []byte("retransmission")) {
		t.Fatalf("unexpected prompt %q", prompt)
	}
}

func TestBuildBatchPromptPreservesWrapperFields(t *testing.T) {
	prompt := buildBatchPrompt([]capture.Record{{
		Number:         1,
		Time:           "2026-04-18T00:00:00Z",
		Src:            "100.64.0.1",
		Dst:            "100.64.0.2",
		Protocol:       "TCP",
		Info:           "1234 -> 443",
		PathID:         7,
		SNAT:           "100.64.0.10",
		DNAT:           "100.64.0.20",
		PayloadPreview: "GET /health",
	}})
	for _, needle := range []string{"path_id=7", "snat=100.64.0.10", "dnat=100.64.0.20", `payload="GET /health"`} {
		if !strings.Contains(prompt, needle) {
			t.Fatalf("expected prompt to contain %q, got %q", needle, prompt)
		}
	}
}

func TestWebPresenterServesSnapshotAndEvents(t *testing.T) {
	state := NewStateStore(Config{Model: "gpt-4o"})
	presenter := NewWebPresenter()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- presenter.Run(ctx, state, func(context.Context) error {
			state.Update(func(snapshot *SessionSnapshot) {
				snapshot.Status = "analysis updated"
				snapshot.Phase = "collecting"
				snapshot.Analysis = []string{"browser output"}
				pushSnapshotEvent(snapshot, "browser output")
			})
			time.Sleep(100 * time.Millisecond)
			return nil
		})
	}()

	url := <-presenter.Ready()
	if !IsLocalhostURL(url) {
		t.Fatalf("expected localhost url, got %q", url)
	}

	resp, err := http.Get(url + "/snapshot")
	if err != nil {
		t.Fatalf("snapshot request error = %v", err)
	}
	defer resp.Body.Close()
	var snapshot SessionSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if snapshot.Model != "gpt-4o" {
		t.Fatalf("expected model in snapshot, got %#v", snapshot)
	}

	eventsResp, err := http.Get(url + "/events")
	if err != nil {
		t.Fatalf("events request error = %v", err)
	}
	defer eventsResp.Body.Close()
	body, err := io.ReadAll(eventsResp.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if !bytes.Contains(body, []byte("event: snapshot")) {
		t.Fatalf("expected sse snapshot event, got %q", body)
	}

	if err := <-errCh; err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	_, err = http.Get(url + "/snapshot")
	if err == nil {
		t.Fatal("expected server shutdown after session completion")
	}
}

func waitForSnapshot(t *testing.T, state *StateStore, ok func(SessionSnapshot) bool) SessionSnapshot {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		snapshot := state.Snapshot()
		if ok(snapshot) {
			return snapshot
		}
		time.Sleep(10 * time.Millisecond)
	}

	snapshot := state.Snapshot()
	t.Fatalf("condition not met, last snapshot: %#v", snapshot)
	return SessionSnapshot{}
}
