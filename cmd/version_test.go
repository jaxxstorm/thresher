package cmd

import (
	"strings"
	"testing"
)

func TestVersionCommandPrintsVersion(t *testing.T) {
	output, err := executeCommand("version")
	if err != nil {
		t.Fatalf("Execute(version) error = %v", err)
	}

	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		t.Fatal("expected non-empty version output")
	}

	if !strings.Contains(trimmed, "v") {
		t.Fatalf("expected version output to contain v prefix, got %q", trimmed)
	}
}
