package analyze

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

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
	input := bytes.NewBufferString("{\"number\":1,\"time\":\"2026-04-18T00:00:00Z\",\"src\":\"100.64.0.1\",\"dst\":\"100.64.0.2\",\"protocol\":\"TCP\",\"length\":64,\"info\":\"1234 -> 443\"}\n")
	if err := session.RunReader(context.Background(), input); err != nil {
		t.Fatalf("RunReader() error = %v", err)
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
