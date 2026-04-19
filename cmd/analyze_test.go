package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestAnalyzeAppearsInRootHelp(t *testing.T) {
	output, err := executeCommand("--help")
	if err != nil {
		t.Fatalf("executeCommand(--help) error = %v", err)
	}
	if !strings.Contains(output, "analyze") {
		t.Fatalf("expected analyze in help output, got %q", output)
	}
}

func TestAnalyzeUsesBuiltInEndpointDefault(t *testing.T) {
	original := openAnalyzeCaptureStream
	openAnalyzeCaptureStream = func(context.Context) (io.ReadCloser, error) {
		return nil, errors.New("stop")
	}
	defer func() { openAnalyzeCaptureStream = original }()

	resetAnalyzeStateForTest()
	viper.Set("analyze.model", "gpt-4o")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runAnalyze(context.Background(), &stdout, &stderr)
	if err == nil {
		t.Fatal("expected stream stop error")
	}
	if !strings.Contains(stderr.String(), "endpoint=http://ai model=gpt-4o mode=console") {
		t.Fatalf("expected built-in endpoint default, got %q", stderr.String())
	}
}

func TestAnalyzeConfigEndpointOverridesBuiltInDefault(t *testing.T) {
	original := openAnalyzeCaptureStream
	openAnalyzeCaptureStream = func(context.Context) (io.ReadCloser, error) {
		return nil, errors.New("stop")
	}
	defer func() { openAnalyzeCaptureStream = original }()

	resetAnalyzeStateForTest()
	viper.Set("analyze.endpoint", "http://configured")
	viper.Set("analyze.model", "configured-model")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runAnalyze(context.Background(), &stdout, &stderr)
	if err == nil {
		t.Fatal("expected stream stop error")
	}
	if !strings.Contains(stderr.String(), "endpoint=http://configured model=configured-model mode=console") {
		t.Fatalf("expected configured values in output, got %q", stderr.String())
	}
}

func TestAnalyzeFlagOverridesConfig(t *testing.T) {
	original := openAnalyzeCaptureStream
	openAnalyzeCaptureStream = func(context.Context) (io.ReadCloser, error) {
		return nil, errors.New("stop")
	}
	defer func() { openAnalyzeCaptureStream = original }()

	resetAnalyzeStateForTest()
	viper.Set("analyze.endpoint", "http://configured")
	viper.Set("analyze.model", "configured-model")
	analyzeArgs.endpoint = "http://flagged"
	analyzeArgs.model = "flagged-model"

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runAnalyze(context.Background(), &stdout, &stderr)
	if err == nil {
		t.Fatal("expected stream stop error")
	}
	if !strings.Contains(stderr.String(), "endpoint=http://flagged model=flagged-model mode=console") {
		t.Fatalf("expected flagged values in output, got %q", stderr.String())
	}
}

func TestAnalyzeDefaultsToConsoleMode(t *testing.T) {
	original := openAnalyzeCaptureStream
	openAnalyzeCaptureStream = func(context.Context) (io.ReadCloser, error) {
		return nil, errors.New("stop")
	}
	defer func() { openAnalyzeCaptureStream = original }()

	resetAnalyzeStateForTest()
	viper.Set("analyze.model", "gpt-4o")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runAnalyze(context.Background(), &stdout, &stderr)
	if err == nil {
		t.Fatal("expected stream stop error")
	}
	if !strings.Contains(stderr.String(), "mode=console") {
		t.Fatalf("expected default console mode, got %q", stderr.String())
	}
}

func TestAnalyzeSupportsExplicitWebMode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			_, _ = w.Write([]byte(`{"data":[{"id":"configured-model"}]}`))
		case "/v1/chat/completions":
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"saved-input analysis"}}]}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "capture.jsonl")
	content := "{\"number\":1,\"time\":\"2026-04-18T00:00:00Z\",\"src\":\"100.64.0.1\",\"dst\":\"100.64.0.2\",\"protocol\":\"TCP\",\"length\":64,\"info\":\"1234 -> 443\"}\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	output, err := executeCommand("analyze", "--endpoint", server.URL, "--model", "configured-model", "--mode", "web", "--input", path)
	if err != nil {
		t.Fatalf("executeCommand(analyze --mode web --input) error = %v", err)
	}
	if !strings.Contains(output, "mode=web url=http://127.0.0.1:") {
		t.Fatalf("expected web startup output, got %q", output)
	}
}

func TestAnalyzeWebModePreservesWrapperFieldsAndSessionLimits(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			_, _ = w.Write([]byte(`{"data":[{"id":"configured-model"}]}`))
		case "/v1/chat/completions":
			requests++
			var payload struct {
				Messages []struct {
					Content string `json:"content"`
				} `json:"messages"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if len(payload.Messages) != 1 {
				t.Fatalf("expected a single chat message, got %#v", payload)
			}
			for _, needle := range []string{"path_id=7", "snat=100.64.0.10", "dnat=100.64.0.20", `payload="GET /health"`} {
				if !strings.Contains(payload.Messages[0].Content, needle) {
					t.Fatalf("expected prompt to contain %q, got %q", needle, payload.Messages[0].Content)
				}
			}
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"saved-input analysis"}}]}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "capture.jsonl")
	content := strings.Join([]string{
		`{"number":1,"time":"2026-04-18T00:00:00Z","src":"100.64.0.1","dst":"100.64.0.2","protocol":"TCP","length":64,"info":"1234 -> 443","path_id":7,"snat":"100.64.0.10","dnat":"100.64.0.20","payload_preview":"GET /health"}`,
		`{"number":2,"time":"2026-04-18T00:00:01Z","src":"100.64.0.2","dst":"100.64.0.1","protocol":"TCP","length":64,"info":"443 -> 1234","path_id":8,"payload_preview":"HTTP/1.1 200 OK"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	output, err := executeCommand(
		"analyze",
		"--endpoint", server.URL,
		"--model", "configured-model",
		"--mode", "web",
		"--batch-packets", "1",
		"--session-packets", "1",
		"--input", path,
	)
	if err != nil {
		t.Fatalf("executeCommand(analyze web wrapper fields) error = %v", err)
	}
	if !strings.Contains(output, "mode=web url=http://127.0.0.1:") {
		t.Fatalf("expected web startup output, got %q", output)
	}
	if requests != 1 {
		t.Fatalf("expected session packet limit to allow one upload, got %d requests", requests)
	}
}

func TestAnalyzeRejectsInvalidMode(t *testing.T) {
	resetAnalyzeStateForTest()
	viper.Set("analyze.model", "gpt-4o")
	analyzeArgs.mode = "browser"

	err := runAnalyze(context.Background(), io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected invalid mode error")
	}
	if !strings.Contains(err.Error(), "invalid analysis mode") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestAnalyseAliasRegistered(t *testing.T) {
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"help", "analyse"})
	err := ExecuteContext(context.Background())
	rootCmd.SetArgs(nil)
	if err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	if !strings.Contains(buf.String(), "analyze") {
		t.Fatalf("expected alias to resolve to analyze help, got %q", buf.String())
	}
}

func TestAnalyzeSupportsSavedInput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			_, _ = w.Write([]byte(`{"data":[{"id":"configured-model"}]}`))
		case "/v1/chat/completions":
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"saved-input analysis"}}]}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "capture.jsonl")
	content := "{\"number\":1,\"time\":\"2026-04-18T00:00:00Z\",\"src\":\"100.64.0.1\",\"dst\":\"100.64.0.2\",\"protocol\":\"TCP\",\"length\":64,\"info\":\"1234 -> 443\"}\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	output, err := executeCommand("analyze", "--endpoint", server.URL, "--model", "configured-model", "--input", path)
	if err != nil {
		t.Fatalf("executeCommand(analyze --input) error = %v", err)
	}
	if !strings.Contains(output, "analyze started") {
		t.Fatalf("expected analyze startup output, got %q", output)
	}
}

func TestInteractiveAnalyzeSessionRequiresRealTTYs(t *testing.T) {
	if isInteractiveAnalyzeSession() {
		t.Fatal("expected test process stdio to be non-interactive")
	}
}

func resetAnalyzeStateForTest() {
	viper.Reset()
	setAnalyzeDefaults()
	analyzeArgs.endpoint = ""
	analyzeArgs.model = ""
	analyzeArgs.input = ""
	analyzeArgs.mode = ""
	analyzeArgs.endpointStyle = ""
	analyzeArgs.batchPackets = 0
	analyzeArgs.batchBytes = 0
	analyzeArgs.sessionPackets = 0
	analyzeArgs.sessionBytes = 0
	analyzeArgs.maxTokens = 0
}
