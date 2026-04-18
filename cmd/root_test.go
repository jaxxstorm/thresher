package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func executeCommand(args ...string) (string, error) {
	return executeCommandContext(context.Background(), args...)
}

func executeCommandContext(ctx context.Context, args ...string) (string, error) {
	buf := &bytes.Buffer{}
	viper.Reset()
	captureArgs.output = ""
	captureArgs.format = "jsonl"
	analyzeArgs.endpoint = ""
	analyzeArgs.model = ""
	analyzeArgs.input = ""
	analyzeArgs.endpointStyle = ""
	analyzeArgs.batchPackets = 0
	analyzeArgs.batchBytes = 0
	analyzeArgs.sessionPackets = 0
	analyzeArgs.sessionBytes = 0
	analyzeArgs.maxTokens = 0
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(args)

	err := ExecuteContext(ctx)

	rootCmd.SetArgs(nil)

	return buf.String(), err
}

func TestRootCommandShowsHelpInsteadOfRunning(t *testing.T) {
	output, err := executeCommand()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if strings.TrimSpace(output) == "" {
		t.Fatal("expected non-empty output")
	}

	if strings.Contains(output, "hello world") {
		t.Fatalf("did not expect banner output, got %q", output)
	}
	if !strings.Contains(output, "capture") {
		t.Fatalf("expected help output mentioning capture, got %q", output)
	}
}

func TestInitConfigReadsConfigFile(t *testing.T) {
	viper.Reset()
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	configPath := filepath.Join(dir, "thresher.yaml")
	if err := os.WriteFile(configPath, []byte("analyze:\n  endpoint: http://configured\n  model: configured-model\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	initConfig()
	if got := viper.GetString("analyze.endpoint"); got != "http://configured" {
		t.Fatalf("expected configured endpoint, got %q", got)
	}
}
