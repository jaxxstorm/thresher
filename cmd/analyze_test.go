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

	"github.com/jaxxstorm/thresher/internal/analyze"
	"github.com/spf13/viper"
)

type stubAnalyzeWebPresenter struct {
	url string
}

func (p *stubAnalyzeWebPresenter) Ready() <-chan string {
	ch := make(chan string, 1)
	ch <- p.url
	return ch
}

func (p *stubAnalyzeWebPresenter) Run(ctx context.Context, state *analyze.StateStore, worker func(context.Context) error) error {
	return worker(ctx)
}

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

func TestAnalyzeSupportsExplicitWebSubcommand(t *testing.T) {
	resetAnalyzeStateForTest()
	originalPresenterFactory := newAnalyzeWebPresenter
	t.Cleanup(func() { newAnalyzeWebPresenter = originalPresenterFactory })
	newAnalyzeWebPresenter = func(config analyze.Config) analyzeWebPresenter {
		if config.WebAccess != analyze.WebAccessLocal {
			t.Fatalf("expected default web access to be local, got %q", config.WebAccess)
		}
		return &stubAnalyzeWebPresenter{url: "http://127.0.0.1:41001"}
	}

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

	output, err := executeCommand("analyze", "web", "--endpoint", server.URL, "--model", "configured-model", "--input", path)
	if err != nil {
		t.Fatalf("executeCommand(analyze web --input) error = %v", err)
	}
	if !strings.Contains(output, "mode=web web-access=local url=http://127.0.0.1:41001") {
		t.Fatalf("expected web startup output, got %q", output)
	}
}

func TestAnalyzeWebSubcommandSupportsExplicitTailnetAccess(t *testing.T) {
	resetAnalyzeStateForTest()
	originalPresenterFactory := newAnalyzeWebPresenter
	t.Cleanup(func() { newAnalyzeWebPresenter = originalPresenterFactory })
	newAnalyzeWebPresenter = func(config analyze.Config) analyzeWebPresenter {
		if config.WebAccess != analyze.WebAccessTailnet {
			t.Fatalf("expected explicit tailnet web access, got %q", config.WebAccess)
		}
		return &stubAnalyzeWebPresenter{url: "http://thresher.tail.ts.net:41234"}
	}

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

	output, err := executeCommand("analyze", "web", "--endpoint", server.URL, "--model", "configured-model", "--web-access", "tailnet", "--input", path)
	if err != nil {
		t.Fatalf("executeCommand(analyze web --web-access tailnet --input) error = %v", err)
	}
	if !strings.Contains(output, "mode=web web-access=tailnet url=http://thresher.tail.ts.net:41234") {
		t.Fatalf("expected tailnet startup output, got %q", output)
	}
}

func TestAnalyzeWebSubcommandPreservesWrapperFieldsAndSessionLimits(t *testing.T) {
	resetAnalyzeStateForTest()
	originalPresenterFactory := newAnalyzeWebPresenter
	t.Cleanup(func() { newAnalyzeWebPresenter = originalPresenterFactory })
	newAnalyzeWebPresenter = func(config analyze.Config) analyzeWebPresenter {
		if config.WebAccess != analyze.WebAccessLocal {
			t.Fatalf("expected wrapper-field web test to stay local, got %q", config.WebAccess)
		}
		return &stubAnalyzeWebPresenter{url: "http://127.0.0.1:41002"}
	}

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
		"web",
		"--endpoint", server.URL,
		"--model", "configured-model",
		"--batch-packets", "1",
		"--session-packets", "1",
		"--input", path,
	)
	if err != nil {
		t.Fatalf("executeCommand(analyze web wrapper fields) error = %v", err)
	}
	if !strings.Contains(output, "mode=web web-access=local url=http://127.0.0.1:41002") {
		t.Fatalf("expected web startup output, got %q", output)
	}
	if requests != 1 {
		t.Fatalf("expected session packet limit to allow one upload, got %d requests", requests)
	}
}

func TestAnalyzeConsoleSubcommandMatchesBareAnalyze(t *testing.T) {
	resetAnalyzeStateForTest()
	original := openAnalyzeCaptureStream
	openAnalyzeCaptureStream = func(context.Context) (io.ReadCloser, error) {
		return nil, errors.New("stop")
	}
	defer func() { openAnalyzeCaptureStream = original }()

	bareOutput, bareErr := executeCommand("analyze", "--model", "gpt-4o")
	if bareErr == nil {
		t.Fatal("expected bare analyze to stop with stream error")
	}
	consoleOutput, consoleErr := executeCommand("analyze", "console", "--model", "gpt-4o")
	if consoleErr == nil {
		t.Fatal("expected analyze console to stop with stream error")
	}
	if !strings.Contains(bareOutput, "mode=console") || !strings.Contains(consoleOutput, "mode=console") {
		t.Fatalf("expected both console entrypoints to report console mode, bare=%q console=%q", bareOutput, consoleOutput)
	}
}

func TestAnalyzeLocalAliasMatchesConsoleWorkflow(t *testing.T) {
	resetAnalyzeStateForTest()
	original := openAnalyzeCaptureStream
	openAnalyzeCaptureStream = func(context.Context) (io.ReadCloser, error) {
		return nil, errors.New("stop")
	}
	defer func() { openAnalyzeCaptureStream = original }()

	output, err := executeCommand("analyze", "local", "--model", "gpt-4o")
	if err == nil {
		t.Fatal("expected analyze local to stop with stream error")
	}
	if !strings.Contains(output, "mode=console") {
		t.Fatalf("expected local alias to resolve to console workflow, got %q", output)
	}
}

func TestAnalyzeRejectsModeFlag(t *testing.T) {
	resetAnalyzeStateForTest()
	_, err := executeCommand("analyze", "--mode", "web")
	if err == nil {
		t.Fatal("expected unknown mode flag error")
	}
	if !strings.Contains(err.Error(), "unknown flag: --mode") {
		t.Fatalf("expected unknown flag error, got %v", err)
	}
}

func TestAnalyzeConsoleRejectsWebOnlyFlag(t *testing.T) {
	resetAnalyzeStateForTest()
	_, err := executeCommand("analyze", "console", "--web-access", "tailnet")
	if err == nil {
		t.Fatal("expected unknown web-access flag error")
	}
	if !strings.Contains(err.Error(), "unknown flag: --web-access") {
		t.Fatalf("expected unknown flag error, got %v", err)
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
	resetAnalyzeStateForTest()
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
	analyzeArgs.webAccess = ""
	analyzeArgs.endpointStyle = ""
	analyzeArgs.batchPackets = 0
	analyzeArgs.batchBytes = 0
	analyzeArgs.sessionPackets = 0
	analyzeArgs.sessionBytes = 0
	analyzeArgs.maxTokens = 0
}
