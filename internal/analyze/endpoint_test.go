package analyze

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jaxxstorm/thresher/internal/capture"
)

func TestAutoEndpointDiscovery(t *testing.T) {
	for _, tc := range []struct {
		name   string
		style  EndpointStyle
		models string
		status int
		want   []string
	}{
		{"model switch", EndpointAuto, `{"data":[{"id":"first","supported_endpoints":["/v1/messages"]},{"id":"second","supported_endpoints":["/v1/responses"]}]}`, 200, []string{"/v1/messages", "/v1/responses"}},
		{"empty style", "", `{"data":[{"id":"first","supported_endpoints":["/v1/messages"]}]}`, 200, []string{"/v1/messages", "/v1/chat/completions"}},
		{"explicit override", EndpointChatCompletions, `{"data":[{"id":"first","supported_endpoints":["/v1/messages"]}]}`, 200, []string{"/v1/chat/completions", "/v1/chat/completions"}},
		{"preference", EndpointAuto, `{"data":[{"id":"first","supported_endpoints":["/v1/messages","/v1/responses","/v1/chat/completions"]},{"id":"second","supported_endpoints":["/v1/messages","/v1/responses"]}]}`, 200, []string{"/v1/chat/completions", "/v1/responses"}},
		{"missing capabilities", EndpointAuto, `{"data":[{"id":"first"},{"id":"second","supported_endpoints":[]}]}`, 200, []string{"/v1/chat/completions", "/v1/chat/completions"}},
		{"unknown capabilities", EndpointAuto, `{"data":[{"id":"first","supported_endpoints":["/unsupported"]}]}`, 200, []string{"/v1/chat/completions", "/v1/chat/completions"}},
		{"discovery unavailable", EndpointAuto, `unavailable`, 503, []string{"/v1/chat/completions", "/v1/chat/completions"}},
		{"malformed discovery", EndpointAuto, `{`, 200, []string{"/v1/chat/completions", "/v1/chat/completions"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			type request struct {
				path, header string
				body         map[string]any
			}
			requests := make(chan request, 3)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v1/models" {
					requests <- request{path: r.URL.Path, header: r.Header.Get("Session_id")}
					w.WriteHeader(tc.status)
					_, _ = io.WriteString(w, tc.models)
					return
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				requests <- request{r.URL.Path, r.Header.Get("Session_id"), body}
				switch r.URL.Path {
				case "/v1/messages":
					_, _ = io.WriteString(w, `{"content":[{"text":"analysis"}]}`)
				case "/v1/responses":
					_, _ = io.WriteString(w, `{"output":[{"content":[{"text":"analysis"}]}]}`)
				default:
					_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"analysis"}}]}`)
				}
			}))
			defer server.Close()
			session := NewSession(Config{Endpoint: server.URL, EndpointStyle: tc.style, Model: "first", BatchPackets: 1})
			session.loadModels(context.Background())
			if discovery := <-requests; discovery.path != "/v1/models" || discovery.header != "" {
				t.Fatalf("unexpected discovery request: %#v", discovery)
			}
			for i, model := range []string{"first", "second"} {
				session.State().SetActiveModel(model)
				if err := session.consumeRecord(context.Background(), capture.Record{Number: i + 1, Info: "synthetic routing test"}); err != nil {
					t.Fatal(err)
				}
				got := <-requests
				if got.path != tc.want[i] || got.body["model"] != model {
					t.Fatalf("request = %#v, want path %q model %q", got, tc.want[i], model)
				}
				switch got.path {
				case "/v1/messages":
					metadata, _ := got.body["metadata"].(map[string]any)
					if metadata["user_id"] != session.id || got.header != "" {
						t.Fatalf("unexpected messages identity: %#v", got)
					}
				case "/v1/responses":
					if got.header != session.id {
						t.Fatalf("unexpected responses identity %q", got.header)
					}
				default:
					if got.header != sessionFingerprint(session.id) {
						t.Fatalf("unexpected chat identity %q", got.header)
					}
				}
			}
			if len(requests) != 0 || session.State().Snapshot().UploadedBatches != 2 {
				t.Fatal("expected two uploads and no extra discovery requests")
			}
		})
	}
}
