package cmd

import (
	"bytes"
	"context"
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
	if !strings.Contains(stderr.String(), "endpoint=http://ai model=gpt-4o") {
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
	if !strings.Contains(stderr.String(), "endpoint=http://configured model=configured-model") {
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
	if !strings.Contains(stderr.String(), "endpoint=http://flagged model=flagged-model") {
		t.Fatalf("expected flagged values in output, got %q", stderr.String())
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
	analyzeArgs.endpointStyle = ""
	analyzeArgs.batchPackets = 0
	analyzeArgs.batchBytes = 0
	analyzeArgs.sessionPackets = 0
	analyzeArgs.sessionBytes = 0
	analyzeArgs.maxTokens = 0
}
