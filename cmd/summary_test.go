package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jaxxstorm/thresher/internal/analyze"
	"github.com/spf13/cobra"
)

func assertSummaryDocument(t *testing.T, data []byte, packets int) {
	t.Helper()
	var document struct {
		SchemaVersion int    `json:"schema_version"`
		Mode          string `json:"mode"`
		Capture       struct {
			PacketCount int `json:"packet_count"`
		} `json:"capture"`
		Packets json.RawMessage `json:"packets"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("invalid summary JSON: %v\n%s", err, data)
	}
	if document.SchemaVersion != 1 || document.Mode != "summary" || document.Capture.PacketCount != packets || document.Packets != nil {
		t.Fatalf("expected aggregate summary of %d packets, got %s", packets, data)
	}
	for _, forbidden := range []string{`"raw_hex"`, `"payload_hex"`, `"payload_preview"`} {
		if bytes.Contains(data, []byte(forbidden)) {
			t.Errorf("summary contains packet detail %s: %s", forbidden, data)
		}
	}
}

func executeSummaryConvert(t *testing.T, args ...string) (string, error) {
	t.Helper()
	original := summary
	defer func() { summary = original }()
	// Convert's closure-backed flags otherwise retain values across executions.
	root := &cobra.Command{Use: "thresher", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().BoolVar(&summary, "summary", false, "use aggregated JSON")
	root.AddCommand(newConvertCommand())
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(io.Discard)
	root.SetArgs(args)
	err := root.Execute()
	return stdout.String(), err
}

func TestGlobalSummaryCapture(t *testing.T) {
	data := testPCAP(t, [][]byte{wrappedTCPPacket(t), wrappedTCPPacket(t)})
	original := openCaptureStream
	t.Cleanup(func() { openCaptureStream = original })
	openCaptureStream = func(context.Context) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(data)), nil
	}
	for _, position := range []string{"before", "after"} {
		for _, destination := range []string{"stdout", "file"} {
			t.Run(position+"/"+destination, func(t *testing.T) {
				args := []string{"capture", "--summary"}
				if position == "before" {
					args = []string{"--summary", "capture"}
				}
				path := filepath.Join(t.TempDir(), "capture.json")
				if destination == "file" {
					args = append(args, "-o", path)
				}
				output, err := executeCommand(args...)
				if err != nil {
					t.Fatal(err)
				}
				status, body, ok := strings.Cut(output, "\n")
				if !ok || !strings.Contains(status, "capture started") {
					t.Fatalf("missing capture startup status: %q", output)
				}
				if destination == "file" {
					if body != "" {
						t.Fatalf("file capture wrote JSON to stdout: %q", body)
					}
					data, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					body = string(data)
				}
				assertSummaryDocument(t, []byte(body), 2)
			})
		}
	}
}

func TestGlobalSummaryConvert(t *testing.T) {
	for _, position := range []string{"before", "after"} {
		for _, destination := range []string{"stdout", "default", "custom"} {
			t.Run(position+"/"+destination, func(t *testing.T) {
				dir := t.TempDir()
				input := filepath.Join(dir, "capture.pcap")
				if err := os.WriteFile(input, testPCAP(t, [][]byte{wrappedTCPPacket(t), wrappedTCPPacket(t)}), 0o600); err != nil {
					t.Fatal(err)
				}
				args := []string{"convert", input, "--summary"}
				if position == "before" {
					args = []string{"--summary", "convert", input}
				}
				path := filepath.Join(dir, "capture.json")
				switch destination {
				case "stdout":
					args = append(args, "-o", "-")
				case "custom":
					path = filepath.Join(dir, "chosen.json")
					args = append(args, "-o", path)
				}
				output, err := executeSummaryConvert(t, args...)
				if err != nil {
					t.Fatal(err)
				}
				if destination == "stdout" {
					if _, err := os.Stat(path); !os.IsNotExist(err) {
						t.Fatalf("stdout conversion created output file: %v", err)
					}
				} else {
					if output != "" {
						t.Fatalf("file conversion wrote stdout: %q", output)
					}
					data, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					output = string(data)
				}
				assertSummaryDocument(t, []byte(output), 2)
			})
		}
	}
}

func TestGlobalSummaryConvertRawConflictBeforeIO(t *testing.T) {
	for _, position := range []string{"before", "after"} {
		for _, raw := range []string{"--include-raw", "--include-raw=true"} {
			for _, destination := range []string{"existing", "absent", "stdout"} {
				t.Run(position+"/"+raw+"/"+destination, func(t *testing.T) {
					dir := t.TempDir()
					path := filepath.Join(dir, "out.json")
					if destination == "existing" {
						if err := os.WriteFile(path, []byte("preserve me"), 0o600); err != nil {
							t.Fatal(err)
						}
					} else if destination == "stdout" {
						path = "-"
					}
					// Missing input proves validation occurs before opening the PCAP.
					args := []string{"convert", filepath.Join(dir, "missing.pcap"), "-o", path, raw}
					if position == "before" {
						args = append([]string{"--summary"}, args...)
					} else {
						args = append(args, "--summary")
					}
					output, err := executeSummaryConvert(t, args...)
					if err == nil || err.Error() != "--include-raw=true is not supported with --summary" {
						t.Fatalf("expected raw/summary conflict, got %v", err)
					}
					if output != "" {
						t.Fatalf("invalid options wrote stdout: %q", output)
					}
					entries, err := os.ReadDir(dir)
					if err != nil {
						t.Fatal(err)
					}
					if destination == "existing" {
						data, err := os.ReadFile(path)
						if err != nil || string(data) != "preserve me" || len(entries) != 1 {
							t.Fatalf("invalid options modified output or leaked files: %v", err)
						}
					} else if len(entries) != 0 {
						t.Fatalf("invalid options created files: %v", entries)
					}
				})
			}
		}
	}
}

func TestGlobalSummaryAnalyzeRequest(t *testing.T) {
	original := newAnalyzeWebPresenter
	t.Cleanup(func() { newAnalyzeWebPresenter = original })
	newAnalyzeWebPresenter = func(config analyze.Config) analyzeWebPresenter {
		if !config.Summary {
			t.Error("web presenter did not inherit summary configuration")
		}
		return &stubAnalyzeWebPresenter{url: "http://127.0.0.1:41001"}
	}
	for _, mode := range []string{"default", "console", "web"} {
		for _, position := range []string{"before", "after"} {
			t.Run(mode+"/"+position, func(t *testing.T) {
				var requests []string
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/v1/models":
						_, _ = io.WriteString(w, `{"data":[{"id":"test-model"}]}`)
					case "/v1/chat/completions":
						var body struct {
							Messages []struct {
								Content string `json:"content"`
							} `json:"messages"`
						}
						if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
							t.Error(err)
						} else if len(body.Messages) != 1 {
							t.Errorf("expected one message, got %d", len(body.Messages))
						} else {
							requests = append(requests, body.Messages[0].Content)
						}
						_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"summary analysis"}}]}`)
					default:
						t.Errorf("unexpected request path %q", r.URL.Path)
						http.NotFound(w, r)
					}
				}))
				defer server.Close()
				path := filepath.Join(t.TempDir(), "capture.jsonl")
				record := `{"number":1,"src":"100.64.0.1","dst":"100.64.0.2","protocol":"TCP","length":64,"info":"packet-detail-marker","payload_preview":"payload-marker","raw_hex":"deadbeef"}` + "\n"
				if err := os.WriteFile(path, []byte(strings.Repeat(record, 2)), 0o600); err != nil {
					t.Fatal(err)
				}
				args := []string{"analyze"}
				if mode != "default" {
					args = append(args, mode)
				}
				args = append(args, "--endpoint", server.URL, "--model", "test-model", "--input", path)
				if position == "before" {
					args = append([]string{"--summary"}, args...)
				} else {
					args = append(args, "--summary")
				}
				if output, err := executeCommand(args...); err != nil {
					t.Fatalf("analyze failed: %v\n%s", err, output)
				}
				server.Close()
				if len(requests) != 1 {
					t.Fatalf("expected one aggregated request, got %d", len(requests))
				}
				prompt := requests[0]
				if !strings.Contains(prompt, "lossy batch summary") {
					t.Fatalf("missing summary context: %s", prompt)
				}
				for _, forbidden := range []string{"packet-detail-marker", "payload-marker", "deadbeef"} {
					if strings.Contains(prompt, forbidden) {
						t.Errorf("summary request contains %q", forbidden)
					}
				}
				start := strings.IndexByte(prompt, '{')
				if start < 0 {
					t.Fatalf("request contains no summary JSON: %s", prompt)
				}
				assertSummaryDocument(t, []byte(prompt[start:]), 2)
			})
		}
	}
}
