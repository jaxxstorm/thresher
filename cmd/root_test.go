package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func executeCommand(args ...string) (string, error) {
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(args)

	err := rootCmd.Execute()

	rootCmd.SetArgs(nil)

	return buf.String(), err
}

func TestRootCommandPrintsBanner(t *testing.T) {
	output, err := executeCommand()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if strings.TrimSpace(output) == "" {
		t.Fatal("expected non-empty output")
	}

	if !strings.Contains(output, "hello world") {
		t.Fatalf("expected output to contain hello world, got %q", output)
	}
}
