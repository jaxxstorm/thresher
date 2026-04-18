package analyze

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jaxxstorm/thresher/internal/capture"
)

type Model struct {
	config   Config
	status   string
	model    string
	models   []ModelInfo
	selected int
	paused   bool
	records  int
	analysis []string
}

type recordMsg capture.Record
type analysisMsg string
type statusMsg string
type modelsLoadedMsg []ModelInfo

func NewModel(config Config) Model {
	return Model{config: config, model: config.Model, status: "waiting for packets"}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "m":
			if len(m.models) > 0 {
				m.selected = (m.selected + 1) % len(m.models)
				m.model = m.models[m.selected].ID
				m.status = fmt.Sprintf("switched model to %s", m.model)
			}
		case "p":
			m.paused = !m.paused
			if m.paused {
				m.status = "analysis paused"
			} else {
				m.status = "analysis resumed"
			}
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case recordMsg:
		m.records++
		m.status = fmt.Sprintf("processed %d packets", m.records)
	case analysisMsg:
		m.analysis = append(m.analysis, string(msg))
		m.status = "analysis updated"
	case statusMsg:
		m.status = string(msg)
	case modelsLoadedMsg:
		m.models = []ModelInfo(msg)
	}
	return m, nil
}

func (m Model) View() string {
	var b strings.Builder
	b.WriteString("thresher analyze\n")
	b.WriteString(fmt.Sprintf("Model: %s\n", m.model))
	b.WriteString(fmt.Sprintf("Status: %s\n", m.status))
	b.WriteString(fmt.Sprintf("Packets: %d\n", m.records))
	b.WriteString("Keys: m switch-model, p pause, q quit\n")
	if len(m.models) > 0 {
		b.WriteString("Available models:\n")
		for _, model := range m.models {
			b.WriteString("- " + model.ID + "\n")
		}
	}
	if len(m.analysis) > 0 {
		b.WriteString("\nAnalysis:\n")
		for _, item := range m.analysis {
			b.WriteString(item + "\n")
		}
	}
	return b.String()
}
