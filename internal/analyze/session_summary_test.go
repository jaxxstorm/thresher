package analyze

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jaxxstorm/thresher/internal/capture"
)

func TestSessionBatchSummary(t *testing.T) {
	for _, summary := range []bool{false, true} {
		name := "default-detailed"
		if summary {
			name = "summary"
		}
		t.Run(name, func(t *testing.T) {
			requests := make(chan string, 5)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v1/models" {
					_, _ = io.WriteString(w, `{"data":[]}`)
					return
				}
				var body struct {
					Messages []struct {
						Content string `json:"content"`
					} `json:"messages"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if len(body.Messages) != 1 {
					t.Errorf("expected one message, got %d", len(body.Messages))
				} else {
					requests <- body.Messages[0].Content
				}
				_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"batch analysis"}}]}`)
			}))
			defer server.Close()

			var input bytes.Buffer
			var records []capture.Record
			totalBytes := 0
			for i := 1; i <= 5; i++ {
				record := capture.Record{
					Number: i, FrameNumber: i, FrameLength: i * 64, Length: i * 64,
					Src: "100.64.0.1", Dst: "100.64.0.2", Protocol: "TCP",
					Info: "packet-detail-marker", PayloadPreview: "payload-preview-marker",
					RawHex: "deadbeefcafebabe",
					Inner:  &capture.Inner{Protocol: "TCP", RawHex: "aabbccddeeff", PayloadHex: "abcdef123456"},
				}
				records = append(records, record)
				encoded, err := json.Marshal(record)
				if err != nil {
					t.Fatal(err)
				}
				totalBytes += len(encoded)
				input.Write(encoded)
				input.WriteByte('\n')
			}
			config := Config{Endpoint: server.URL, Model: "test-model", BatchPackets: 2}
			if summary {
				config.Summary = true
			}
			session := NewSession(config)
			session.programOpts = []tea.ProgramOption{tea.WithInput(nil), tea.WithOutput(io.Discard)}
			if err := session.RunReader(context.Background(), &input); err != nil {
				t.Fatal(err)
			}
			if len(requests) != 3 {
				t.Fatalf("got %d requests, want two full batches and one partial batch", len(requests))
			}
			for batch := 0; batch < 3; batch++ {
				content := <-requests
				window := records[batch*2 : min(batch*2+2, len(records))]
				if !summary {
					want := buildPrompt("You are analyzing decoded Tailscale packet capture output. Explain what is happening, identify notable flows, failures, or unusual behavior, and be concise but informative.", buildBatchPrompt(window))
					if content != want {
						t.Fatalf("default prompt changed: %q", content)
					}
					continue
				}
				if !strings.Contains(content, "lossy batch summary") || !strings.Contains(content, "only this batch, not the entire session") {
					t.Fatalf("missing summary context: %q", content)
				}
				for _, forbidden := range []string{"packet-detail-marker", "payload-preview-marker", "deadbeefcafebabe", "aabbccddeeff", "abcdef123456", `"raw_hex"`, `"payload_hex"`, `"payload_preview"`} {
					if strings.Contains(content, forbidden) {
						t.Errorf("summary request contains %q", forbidden)
					}
				}
				start := strings.IndexByte(content, '{')
				if start < 0 {
					t.Fatal("summary request has no JSON")
				}
				var got, want any
				if err := json.Unmarshal([]byte(content[start:]), &got); err != nil {
					t.Fatal(err)
				}
				expected, err := capture.SummaryJSON(window)
				if err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(expected, &want); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("batch %d summary differs from its records: %s", batch, content)
				}
				var counts struct {
					Capture struct {
						PacketCount   int `json:"packet_count"`
						CapturedBytes int `json:"captured_bytes"`
					} `json:"capture"`
				}
				if err := json.Unmarshal([]byte(content[start:]), &counts); err != nil {
					t.Fatal(err)
				}
				wantBytes := 0
				for _, record := range window {
					wantBytes += record.FrameLength
				}
				if counts.Capture.PacketCount != len(window) || counts.Capture.CapturedBytes != wantBytes {
					t.Fatalf("batch %d counts = %+v, want %d packets / %d bytes", batch, counts.Capture, len(window), wantBytes)
				}
			}
			snapshot := session.State().Snapshot()
			if snapshot.Records != 5 || snapshot.TotalBytes != totalBytes || snapshot.UploadedBatches != 3 || len(snapshot.Analysis) != 3 || snapshot.PendingPackets != 0 || snapshot.PendingBytes != 0 || snapshot.InFlight {
				t.Fatalf("unexpected accounting/UI state: %+v", snapshot)
			}
			if len(session.batch) != 0 || session.count != 5 || session.bytes != totalBytes {
				t.Fatal("batch reset or original record accounting changed")
			}
		})
	}
}
