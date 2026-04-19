package analyze

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jaxxstorm/thresher/internal/capture"
)

func TestModelTracksStructuredSessionState(t *testing.T) {
	model := NewModel(Config{Endpoint: "http://ai", Model: "gpt-4o", BatchPackets: 20, BatchBytes: 65536, SessionPackets: 500, SessionBytes: 2097152})

	model.Update(modelsLoadedMsg([]ModelInfo{{ID: "gpt-4o"}, {ID: "claude"}}))
	model.Update(recordMsg{
		record:       capture.Record{Src: "100.64.0.1", Dst: "100.64.0.2"},
		encodedBytes: 128,
	})
	model.Update(batchQueuedMsg{pendingPackets: 1, pendingBytes: 128})
	model.Update(uploadStartedMsg{batchPackets: 1, batchBytes: 128})
	model.Update(analysisMsg{text: "Flow looks healthy", batchPackets: 1, batchBytes: 128})
	model.Update(limitReachedMsg{reason: "analysis limit reached; stopping uploads"})
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})

	snapshot := model.Snapshot()
	if snapshot.Records != 1 {
		t.Fatalf("expected 1 record, got %d", snapshot.Records)
	}
	if snapshot.TotalBytes != 128 {
		t.Fatalf("expected total bytes to be tracked, got %d", snapshot.TotalBytes)
	}
	if snapshot.UploadedBatches != 1 {
		t.Fatalf("expected uploaded batch count, got %d", snapshot.UploadedBatches)
	}
	if !snapshot.LimitReached {
		t.Fatal("expected limit reached state to be tracked")
	}
	if snapshot.Phase != "paused" {
		t.Fatalf("expected paused phase after key toggle, got %q", snapshot.Phase)
	}
	if len(snapshot.Models) != 2 {
		t.Fatalf("expected discovered models, got %#v", snapshot.Models)
	}
	if len(snapshot.Analysis) != 1 || snapshot.Analysis[0] != "Flow looks healthy" {
		t.Fatalf("expected analysis history, got %#v", snapshot.Analysis)
	}
	if snapshot.SelectedModel != "gpt-4o" {
		t.Fatalf("expected selected model to track active model, got %#v", snapshot)
	}
}

func TestModelSelectorAndQuitCancelSession(t *testing.T) {
	model := NewModel(Config{Endpoint: "http://ai", Model: "gpt-4o"})

	model.Update(modelsLoadedMsg([]ModelInfo{{ID: "gpt-4o"}, {ID: "claude"}}))
	model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model.Update(tea.KeyMsg{Type: tea.KeyDown})
	snapshot := model.Snapshot()
	if snapshot.SelectedModel != "claude" {
		t.Fatalf("expected selector to move to claude, got %#v", snapshot)
	}
	if snapshot.Model != "gpt-4o" {
		t.Fatalf("expected active model to remain unchanged before apply, got %#v", snapshot)
	}

	model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	snapshot = model.Snapshot()
	if snapshot.Model != "claude" {
		t.Fatalf("expected enter to apply selected model, got %#v", snapshot)
	}

	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if !model.QuitRequested() {
		t.Fatal("expected quit to mark the session for exit")
	}
}

func TestAnalysisPaneWrapsAndScrolls(t *testing.T) {
	model := NewModel(Config{Endpoint: "http://ai", Model: "gpt-4o"})
	model.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	lines := make([]string, 0, 40)
	for i := 0; i < 40; i++ {
		lines = append(lines, "analysis line "+string(rune('A'+(i%26)))+" wrapped analysis content")
	}
	model.Update(analysisMsg{text: strings.Join(lines, "\n"), batchPackets: 1, batchBytes: 64})

	first := model.View()
	if !strings.Contains(first, "analysis line A wrapped analysis content") {
		t.Fatalf("expected wrapped analysis content to remain visible, got:\n%s", first)
	}

	model.Update(tea.KeyMsg{Type: tea.KeyDown})
	scrolled := model.View()
	if first == scrolled {
		t.Fatalf("expected analysis view to scroll when focused, got unchanged output:\n%s", scrolled)
	}
}

func TestAnalysisPaneFitsWideGlyphs(t *testing.T) {
	model := NewModel(Config{Endpoint: "http://ai", Model: "gpt-4o"})
	model.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	model.Update(analysisMsg{
		text:         "### 🔴 **Failures - Missing Responses**\n- ✅ healthy path still present\n- IPv6 peer `2a05:d014:9b8:3a2b:f1a1:7d8d:e858:3c08` needs wrapping across the pane",
		batchPackets: 1,
		batchBytes:   64,
	})

	view := model.View()
	for _, line := range strings.Split(view, "\n") {
		if got := lipgloss.Width(line); got > 120 {
			t.Fatalf("expected wide-glyph line to fit terminal, got width %d for line %q", got, line)
		}
	}
}

func TestModelViewRendersFullWindowDashboard(t *testing.T) {
	model := NewModel(Config{
		Endpoint:       "http://ai",
		Model:          "gpt-4o",
		BatchPackets:   20,
		BatchBytes:     65536,
		SessionPackets: 500,
		SessionBytes:   2097152,
	})

	model.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	model.Update(modelsLoadedMsg([]ModelInfo{{ID: "gpt-4o"}, {ID: "claude"}}))
	model.Update(recordMsg{
		record:       capture.Record{Src: "100.64.0.1", Dst: "100.64.0.2"},
		encodedBytes: 128,
	})
	model.Update(batchQueuedMsg{pendingPackets: 1, pendingBytes: 128})
	model.Update(uploadStartedMsg{batchPackets: 1, batchBytes: 128})
	model.Update(analysisMsg{text: "Live analysis output", batchPackets: 1, batchBytes: 128})

	view := model.View()
	for _, needle := range []string{
		"THRESHER ANALYZE",
		"Live Analysis",
		"[active]",
		"Counters",
		"Batching",
		"Events",
		"tab focus pane",
		"http://ai",
		"gpt-4o",
		"Live analysis output",
	} {
		if !strings.Contains(view, needle) {
			t.Fatalf("expected view to contain %q, got:\n%s", needle, view)
		}
	}
	if got := lipgloss.Width(view); got > 140 {
		t.Fatalf("expected dashboard width to fit terminal, got width %d", got)
	}
	if got := lipgloss.Height(view); got > 40 {
		t.Fatalf("expected dashboard height to fit terminal, got height %d", got)
	}
}

func TestModelViewStacksForNarrowWindows(t *testing.T) {
	model := NewModel(Config{
		Endpoint:       "http://ai",
		Model:          "gpt-4o",
		BatchPackets:   20,
		BatchBytes:     65536,
		SessionPackets: 500,
		SessionBytes:   2097152,
	})

	model.Update(tea.WindowSizeMsg{Width: 72, Height: 24})
	view := model.View()
	for _, needle := range []string{"Session", "Counters", "Batching", "Live Analysis", "Controls"} {
		if !strings.Contains(view, needle) {
			t.Fatalf("expected narrow view to contain %q, got:\n%s", needle, view)
		}
	}
	if got := lipgloss.Width(view); got > 72 {
		t.Fatalf("expected narrow dashboard width to fit terminal, got width %d", got)
	}
	if got := lipgloss.Height(view); got > 24 {
		t.Fatalf("expected narrow dashboard height to fit terminal, got height %d", got)
	}
}
