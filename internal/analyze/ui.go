package analyze

import (
	"context"
	"fmt"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jaxxstorm/thresher/internal/capture"
	"github.com/mattn/go-runewidth"
)

type Model struct {
	mu              sync.RWMutex
	config          Config
	state           *StateStore
	cancel          context.CancelFunc
	width           int
	height          int
	status          string
	lastEvent       string
	phase           string
	model           string
	models          []ModelInfo
	selected        int
	paused          bool
	records         int
	totalBytes      int
	pendingPackets  int
	pendingBytes    int
	uploadedBatches int
	inFlight        bool
	limitReached    bool
	analysis        []string
	events          []string
	focus           paneFocus
	analysisOffset  int
	modelOffset     int
	quitting        bool
}

type snapshotMsg SessionSnapshot

type paneFocus string

const (
	focusAnalysis paneFocus = "analysis"
	focusModels   paneFocus = "models"
)

type recordMsg struct {
	record       capture.Record
	encodedBytes int
}

type batchQueuedMsg struct {
	pendingPackets int
	pendingBytes   int
}

type uploadStartedMsg struct {
	batchPackets int
	batchBytes   int
}

type analysisMsg struct {
	text         string
	batchPackets int
	batchBytes   int
}

type limitReachedMsg struct {
	reason string
}

type errorMsg struct {
	text string
}

type statusMsg struct {
	text string
}

type modelsLoadedMsg []ModelInfo

func NewModel(config Config) *Model {
	return &Model{
		config:    config,
		model:     config.Model,
		status:    "waiting for packets",
		lastEvent: "session created",
		phase:     "idle",
		focus:     focusAnalysis,
	}
}

func NewBoundModel(config Config, state *StateStore) *Model {
	model := NewModel(config)
	model.state = state
	model.applySnapshot(state.Snapshot())
	return model
}

func (m *Model) Init() tea.Cmd { return nil }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case snapshotMsg:
		m.applySnapshot(SessionSnapshot(msg))
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			if m.focus == focusAnalysis {
				m.focus = focusModels
				m.ensureModelSelectionVisible(0)
			} else {
				m.focus = focusAnalysis
				m.analysisOffset = clamp(m.analysisOffset, 0, m.maxAnalysisOffset())
			}
		case "up", "k":
			if m.focus == focusAnalysis {
				m.analysisOffset = clamp(m.analysisOffset-1, 0, m.maxAnalysisOffset())
			} else if len(m.models) > 0 {
				m.selected--
				if m.selected < 0 {
					m.selected = len(m.models) - 1
				}
				m.ensureModelSelectionVisible(0)
				m.status = fmt.Sprintf("selected model %s", m.models[m.selected].ID)
				m.lastEvent = m.status
			}
		case "down", "j", "m":
			if m.focus == focusAnalysis {
				m.analysisOffset = clamp(m.analysisOffset+1, 0, m.maxAnalysisOffset())
			} else if len(m.models) > 0 {
				m.selected = (m.selected + 1) % len(m.models)
				m.ensureModelSelectionVisible(0)
				m.status = fmt.Sprintf("selected model %s", m.models[m.selected].ID)
				m.lastEvent = m.status
			}
		case "pgup":
			if m.focus == focusAnalysis {
				m.analysisOffset = clamp(m.analysisOffset-5, 0, m.maxAnalysisOffset())
			}
		case "pgdown":
			if m.focus == focusAnalysis {
				m.analysisOffset = clamp(m.analysisOffset+5, 0, m.maxAnalysisOffset())
			}
		case "home":
			if m.focus == focusAnalysis {
				m.analysisOffset = 0
			}
		case "end":
			if m.focus == focusAnalysis {
				m.analysisOffset = m.maxAnalysisOffset()
			}
		case "enter", " ":
			if m.focus == focusModels && len(m.models) > 0 {
				m.model = m.models[m.selected].ID
				m.status = fmt.Sprintf("switched model to %s", m.model)
				m.lastEvent = m.status
				m.pushEvent(m.status)
				if m.state != nil {
					m.state.SetActiveModel(m.model)
				}
				if !m.paused && !m.inFlight && !m.limitReached {
					m.phase = "collecting"
				}
			}
		case "p":
			m.paused = !m.paused
			if m.state != nil {
				m.state.SetPaused(m.paused)
			}
			if m.paused {
				m.status = "analysis paused"
				m.phase = "paused"
			} else {
				m.status = "analysis resumed"
				switch {
				case m.limitReached:
					m.phase = "limited"
				case m.inFlight:
					m.phase = "uploading"
				case m.pendingPackets > 0 || m.records > 0:
					m.phase = "collecting"
				default:
					m.phase = "idle"
				}
			}
			m.lastEvent = m.status
			m.pushEvent(m.status)
		case "q", "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
	case recordMsg:
		m.records++
		m.totalBytes += msg.encodedBytes
		m.status = fmt.Sprintf("processed %d packets", m.records)
		m.lastEvent = fmt.Sprintf("captured packet %d %s -> %s", m.records, msg.record.Src, msg.record.Dst)
		if !m.paused && !m.inFlight && !m.limitReached {
			m.phase = "collecting"
		}
	case batchQueuedMsg:
		m.pendingPackets = msg.pendingPackets
		m.pendingBytes = msg.pendingBytes
		if !m.paused && !m.inFlight && !m.limitReached {
			m.phase = "collecting"
		}
		m.status = fmt.Sprintf("buffering %d packets for the next upload", msg.pendingPackets)
		m.lastEvent = m.status
	case uploadStartedMsg:
		m.pendingPackets = msg.batchPackets
		m.pendingBytes = msg.batchBytes
		m.inFlight = true
		m.phase = "uploading"
		m.status = fmt.Sprintf("uploading batch of %d packets", msg.batchPackets)
		m.lastEvent = m.status
		m.pushEvent(m.status)
	case analysisMsg:
		m.analysis = append(m.analysis, strings.TrimSpace(msg.text))
		m.pendingPackets = 0
		m.pendingBytes = 0
		m.inFlight = false
		m.uploadedBatches++
		m.status = "analysis updated"
		m.lastEvent = fmt.Sprintf("received analysis for %d packets", msg.batchPackets)
		m.pushEvent(m.lastEvent)
		if m.limitReached {
			m.phase = "limited"
		} else if m.paused {
			m.phase = "paused"
		} else {
			m.phase = "collecting"
		}
	case limitReachedMsg:
		m.limitReached = true
		m.inFlight = false
		m.phase = "limited"
		m.status = "analysis limit reached"
		m.lastEvent = msg.reason
		m.pushEvent(msg.reason)
	case errorMsg:
		m.inFlight = false
		m.status = msg.text
		m.lastEvent = msg.text
		m.phase = "error"
		m.pushEvent(msg.text)
	case statusMsg:
		m.status = msg.text
		m.lastEvent = msg.text
		switch {
		case strings.Contains(msg.text, "waiting"):
			m.phase = "idle"
		case strings.Contains(msg.text, "uploading"):
			m.phase = "uploading"
		}
	case modelsLoadedMsg:
		m.models = []ModelInfo(msg)
		m.selected = 0
		for i, model := range m.models {
			if model.ID == m.model {
				m.selected = i
				break
			}
		}
		if len(m.models) > 0 {
			m.lastEvent = fmt.Sprintf("loaded %d models", len(m.models))
			m.pushEvent(m.lastEvent)
		}
	}
	return m, nil
}

func (m *Model) applySnapshot(snapshot SessionSnapshot) {
	m.status = snapshot.Status
	m.lastEvent = snapshot.LastEvent
	m.phase = snapshot.Phase
	m.model = snapshot.Model
	m.records = snapshot.Records
	m.totalBytes = snapshot.TotalBytes
	m.pendingPackets = snapshot.PendingPackets
	m.pendingBytes = snapshot.PendingBytes
	m.uploadedBatches = snapshot.UploadedBatches
	m.inFlight = snapshot.InFlight
	m.limitReached = snapshot.LimitReached
	m.paused = snapshot.Paused
	m.analysis = append([]string(nil), snapshot.Analysis...)
	m.events = append([]string(nil), snapshot.Events...)
	m.models = m.models[:0]
	for _, model := range snapshot.Models {
		m.models = append(m.models, ModelInfo{ID: model})
	}
	if len(m.models) == 0 {
		m.selected = 0
		m.modelOffset = 0
		return
	}

	selected := 0
	for i, model := range m.models {
		if model.ID == m.model {
			selected = i
			break
		}
	}
	m.selected = selected
	m.ensureModelSelectionVisible(0)
}

func (m *Model) View() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.width <= 0 || m.height <= 0 {
		return "thresher analyze\nloading session..."
	}

	palette := viewPalette()
	headerLines := []string{
		palette.header.Render(padRight(" THRESHER ANALYZE ", m.width)),
		palette.subtle.Render(padRight(" "+truncateLine(m.headerSummary(), max(1, m.width-1)), m.width)),
		palette.help.Render(padRight(" tab focus pane  ↑/↓ scroll or select  enter apply  p pause/resume  q quit ", m.width)),
	}

	remainingHeight := max(6, m.height-len(headerLines))
	summaryHeight := 7
	if m.width < 110 {
		summaryHeight = 11
	}
	if summaryHeight > remainingHeight-3 {
		summaryHeight = max(3, remainingHeight/2)
	}
	bodyHeight := max(3, remainingHeight-summaryHeight)

	lines := append([]string{}, headerLines...)
	lines = append(lines, m.renderSummaryLines(m.width, summaryHeight)...)
	lines = append(lines, m.renderBodyLines(m.width, bodyHeight)...)
	return strings.Join(fitOrPadLines(lines, m.height, m.width), "\n")
}

func (m *Model) ActiveModel() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.model
}

func (m *Model) IsPaused() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.paused
}

func (m *Model) QuitRequested() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.quitting
}

type UISnapshot struct {
	SessionSnapshot
	SelectedModel string
}

func (m *Model) Snapshot() UISnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return UISnapshot{
		SessionSnapshot: SessionSnapshot{
			Status:          m.status,
			LastEvent:       m.lastEvent,
			Phase:           m.phase,
			Model:           m.model,
			Records:         m.records,
			TotalBytes:      m.totalBytes,
			PendingPackets:  m.pendingPackets,
			PendingBytes:    m.pendingBytes,
			UploadedBatches: m.uploadedBatches,
			InFlight:        m.inFlight,
			LimitReached:    m.limitReached,
			Paused:          m.paused,
			Models:          modelIDs(m.models),
			Analysis:        append([]string(nil), m.analysis...),
			Events:          append([]string(nil), m.events...),
		},
		SelectedModel: m.selectedModel(),
	}
}

func modelIDs(models []ModelInfo) []string {
	ids := make([]string, 0, len(models))
	for _, model := range models {
		ids = append(ids, model.ID)
	}
	return ids
}

func (m *Model) SetCancel(cancel context.CancelFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cancel = cancel
}

func (m *Model) pushEvent(event string) {
	event = strings.TrimSpace(event)
	if event == "" {
		return
	}
	m.events = append(m.events, event)
	if len(m.events) > 8 {
		m.events = m.events[len(m.events)-8:]
	}
}

func (m *Model) headerSummary() string {
	return fmt.Sprintf("endpoint %s  |  model %s  |  status %s", m.config.Endpoint, m.model, m.status)
}

func (m *Model) renderSummaryLines(width, height int) []string {
	session := []string{
		fmt.Sprintf("State   %s", m.statusChip()),
		fmt.Sprintf("Status  %s", m.status),
		fmt.Sprintf("Event   %s", m.lastEvent),
		fmt.Sprintf("Model   %s", m.model),
		fmt.Sprintf("Target  %s", m.config.Endpoint),
	}
	counters := []string{
		fmt.Sprintf("Packets          %d", m.records),
		fmt.Sprintf("Encoded bytes    %d", m.totalBytes),
		fmt.Sprintf("Session limit    %d packets / %d bytes", m.config.SessionPackets, m.config.SessionBytes),
		fmt.Sprintf("Uploaded batches %d", m.uploadedBatches),
	}
	batching := []string{
		fmt.Sprintf("Pending batch  %d packets / %d bytes", m.pendingPackets, m.pendingBytes),
		fmt.Sprintf("Batch limit    %d packets / %d bytes", m.config.BatchPackets, m.config.BatchBytes),
		fmt.Sprintf("In flight      %t", m.inFlight),
		fmt.Sprintf("Paused         %t", m.paused),
		fmt.Sprintf("Limit reached  %t", m.limitReached),
	}

	if width < 110 {
		cardHeight := max(3, height/3)
		return stackPanels(
			panelLines("Session", session, width, cardHeight),
			panelLines("Counters", counters, width, cardHeight),
			panelLines("Batching", batching, width, max(3, height-(cardHeight*2))),
		)
	}

	gutter := 2
	leftWidth := max(24, (width-(gutter*2))/3)
	middleWidth := max(24, (width-(gutter*2))/3)
	rightWidth := max(24, width-leftWidth-middleWidth-(gutter*2))
	left := joinColumns(
		panelLines("Session", session, leftWidth, height),
		panelLines("Counters", counters, middleWidth, height),
		gutter,
	)
	return joinColumns(
		left,
		panelLines("Batching", batching, rightWidth, height),
		gutter,
	)
}

func (m *Model) renderBodyLines(width, height int) []string {
	analysis := []string{"Waiting for Aperture analysis..."}
	if len(m.analysis) > 0 {
		analysis = flattenBlocks(m.analysis)
	}

	if width < 110 {
		sidebarHeight := min(height-3, max(7, height/3))
		analysisHeight := max(3, height-sidebarHeight)
		return stackPanels(
			m.renderAnalysisPanel(width, analysisHeight, analysis),
			m.renderSidebarPanel(width, sidebarHeight),
		)
	}

	gutter := 2
	sidebarWidth := max(42, width/3)
	analysisWidth := max(40, width-sidebarWidth-gutter)
	return joinColumns(
		m.renderAnalysisPanel(analysisWidth, height, analysis),
		m.renderSidebarPanel(sidebarWidth, height),
		gutter,
	)
}

func (m *Model) modelLines() []string {
	if len(m.models) == 0 {
		return []string{"model discovery unavailable"}
	}

	lines := make([]string, 0, len(m.models))
	for i, model := range m.models {
		cursor := " "
		if i == m.selected {
			cursor = "›"
		}
		active := "○"
		if model.ID == m.model {
			active = "●"
		}
		lines = append(lines, fmt.Sprintf("%s%s %s", cursor, active, model.ID))
	}
	return lines
}

func (m *Model) eventLines() []string {
	if len(m.events) == 0 {
		return []string{"no session events yet"}
	}
	return append([]string(nil), m.events...)
}

func (m *Model) ensureModelSelectionVisible(visibleRows int) {
	if visibleRows <= 0 {
		visibleRows = 8
	}
	if m.selected < m.modelOffset {
		m.modelOffset = m.selected
	}
	if m.selected >= m.modelOffset+visibleRows {
		m.modelOffset = m.selected - visibleRows + 1
	}
	if m.modelOffset < 0 {
		m.modelOffset = 0
	}
}

func (m *Model) maxAnalysisOffset() int {
	if m.width <= 2 || m.height <= 5 {
		return 0
	}
	wrapped := wrapLines(flattenBlocksOrPlaceholder(m.analysis), max(1, analysisPanelWidth(m.width)-2))
	return max(0, len(wrapped)-max(1, analysisPanelHeight(m.width, m.height)-2))
}

func (m *Model) statusChip() string {
	switch m.phase {
	case "uploading":
		return "UPLOADING"
	case "paused":
		return "PAUSED"
	case "limited":
		return "LIMITED"
	case "error":
		return "ERROR"
	case "collecting":
		return "COLLECTING"
	default:
		return "IDLE"
	}
}

func (m *Model) selectedModel() string {
	if len(m.models) == 0 || m.selected < 0 || m.selected >= len(m.models) {
		return ""
	}
	return m.models[m.selected].ID
}

func (m *Model) renderAnalysisPanel(width, height int, analysis []string) []string {
	title := "Live Analysis"
	if m.focus == focusAnalysis {
		title += " [active]"
	}

	wrapped := wrapLines(analysis, max(1, width-2))
	bodyHeight := max(1, height-2)
	maxOffset := max(0, len(wrapped)-bodyHeight)
	m.analysisOffset = clamp(m.analysisOffset, 0, maxOffset)
	visible := sliceWindow(wrapped, m.analysisOffset, bodyHeight)
	return panelLines(title, visible, width, height)
}

func (m *Model) renderSidebarPanel(width, height int) []string {
	title := "Sidebar"
	if m.focus == focusModels {
		title += " [active]"
	}

	bodyHeight := max(1, height-2)
	visibleModels := max(1, bodyHeight-10)
	m.ensureModelSelectionVisible(visibleModels)
	lines := []string{"Models"}
	lines = append(lines, sliceWindow(m.modelLines(), m.modelOffset, visibleModels)...)
	lines = append(lines, "")
	lines = append(lines, "Controls")
	lines = append(lines,
		"tab focus pane",
		"↑/k previous model",
		"↓/j next model",
		"enter apply model",
		"p pause or resume",
		"q or esc quit",
	)
	lines = append(lines, "")
	lines = append(lines, "Events")
	lines = append(lines, m.eventLines()...)
	return panelLines(title, lines, width, height)
}

type palette struct {
	header lipgloss.Style
	subtle lipgloss.Style
	help   lipgloss.Style
}

func viewPalette() palette {
	return palette{
		header: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#F8FAFC")).
			Background(lipgloss.Color("#102A43")),
		subtle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#B8E1E8")),
		help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D9F99D")).
			Background(lipgloss.Color("#183A53")),
	}
}

func flattenBlocks(blocks []string) []string {
	lines := make([]string, 0, len(blocks)*2)
	for _, block := range blocks {
		for _, line := range strings.Split(strings.TrimSpace(block), "\n") {
			lines = append(lines, line)
		}
		lines = append(lines, "")
	}
	if len(lines) > 0 {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func flattenBlocksOrPlaceholder(blocks []string) []string {
	if len(blocks) == 0 {
		return []string{"Waiting for Aperture analysis..."}
	}
	return flattenBlocks(blocks)
}

func wrapLines(lines []string, width int) []string {
	if width <= 0 {
		return nil
	}
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			out = append(out, "")
			continue
		}
		current := ""
		for _, word := range strings.Fields(line) {
			if current == "" {
				current = word
				continue
			}
			if runewidth.StringWidth(current)+1+runewidth.StringWidth(word) <= width {
				current += " " + word
				continue
			}
			out = append(out, truncateChunk(current, width)...)
			current = word
		}
		if current != "" {
			out = append(out, truncateChunk(current, width)...)
		}
	}
	if len(out) == 0 {
		return []string{""}
	}
	return out
}

func truncateChunk(text string, width int) []string {
	if width <= 0 {
		return nil
	}
	if runewidth.StringWidth(text) <= width {
		return []string{text}
	}

	chunks := make([]string, 0, (len([]rune(text))/max(1, width))+1)
	var current strings.Builder
	currentWidth := 0

	for _, r := range text {
		rw := runewidth.RuneWidth(r)
		if rw < 0 {
			rw = 0
		}
		if currentWidth > 0 && currentWidth+rw > width {
			chunks = append(chunks, current.String())
			current.Reset()
			currentWidth = 0
		}
		current.WriteRune(r)
		currentWidth += rw
	}
	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}
	return chunks
}

func sliceWindow(lines []string, offset, height int) []string {
	if height <= 0 || len(lines) == 0 {
		return nil
	}
	offset = clamp(offset, 0, max(0, len(lines)-height))
	end := offset + height
	if end > len(lines) {
		end = len(lines)
	}
	return lines[offset:end]
}

func analysisPanelWidth(totalWidth int) int {
	if totalWidth < 110 {
		return totalWidth
	}
	gutter := 2
	sidebarWidth := max(42, totalWidth/3)
	return max(40, totalWidth-sidebarWidth-gutter)
}

func analysisPanelHeight(totalWidth, totalHeight int) int {
	headerHeight := 3
	remainingHeight := max(6, totalHeight-headerHeight)
	summaryHeight := 7
	if totalWidth < 110 {
		summaryHeight = 11
	}
	if summaryHeight > remainingHeight-3 {
		summaryHeight = max(3, remainingHeight/2)
	}
	bodyHeight := max(3, remainingHeight-summaryHeight)
	if totalWidth < 110 {
		sidebarHeight := min(bodyHeight-3, max(7, bodyHeight/3))
		return max(3, bodyHeight-sidebarHeight)
	}
	return bodyHeight
}

func fitLines(lines []string, maxLines int) []string {
	if maxLines <= 0 {
		return nil
	}
	if len(lines) <= maxLines {
		return lines
	}
	return lines[:maxLines]
}

func truncateLines(lines []string, width int) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, truncateLine(line, width))
	}
	return out
}

func truncateLine(line string, width int) string {
	if width <= 0 {
		return ""
	}
	if runewidth.StringWidth(line) <= width {
		return line
	}
	if width == 1 {
		for _, r := range line {
			return string(r)
		}
		return ""
	}

	var b strings.Builder
	currentWidth := 0
	for _, r := range line {
		rw := runewidth.RuneWidth(r)
		if currentWidth+rw > width-1 {
			break
		}
		b.WriteRune(r)
		currentWidth += rw
	}
	return b.String() + "…"
}

func panelLines(title string, body []string, width, height int) []string {
	width = max(4, width)
	height = max(3, height)
	innerWidth := width - 2
	bodyHeight := height - 2

	titleText := " " + truncateLine(title, max(1, innerWidth-2)) + " "
	fill := innerWidth - runewidth.StringWidth(titleText)
	if fill < 0 {
		fill = 0
	}
	top := "╭" + titleText + strings.Repeat("─", fill) + "╮"
	bottom := "╰" + strings.Repeat("─", innerWidth) + "╯"
	content := fitLines(truncateLines(body, innerWidth), bodyHeight)

	lines := []string{top}
	for i := 0; i < bodyHeight; i++ {
		line := ""
		if i < len(content) {
			line = content[i]
		}
		lines = append(lines, "│"+padRight(line, innerWidth)+"│")
	}
	lines = append(lines, bottom)
	return lines
}

func joinColumns(left, right []string, gutter int) []string {
	height := max(len(left), len(right))
	leftWidth := widestLine(left)
	rightWidth := widestLine(right)
	out := make([]string, 0, height)
	for i := 0; i < height; i++ {
		l := ""
		if i < len(left) {
			l = left[i]
		}
		r := ""
		if i < len(right) {
			r = right[i]
		}
		out = append(out, padRight(l, leftWidth)+strings.Repeat(" ", gutter)+padRight(r, rightWidth))
	}
	return out
}

func stackPanels(parts ...[]string) []string {
	out := make([]string, 0)
	for _, part := range parts {
		out = append(out, part...)
	}
	return out
}

func fitOrPadLines(lines []string, height, width int) []string {
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, padVisible(line, width))
	}
	return out
}

func widestLine(lines []string) int {
	width := 0
	for _, line := range lines {
		if size := lipgloss.Width(line); size > width {
			width = size
		}
	}
	return width
}

func padRight(line string, width int) string {
	if lipgloss.Width(line) >= width {
		return truncateLine(line, width)
	}
	return line + strings.Repeat(" ", width-lipgloss.Width(line))
}

func padVisible(line string, width int) string {
	visible := lipgloss.Width(line)
	if visible >= width {
		return line
	}
	return line + strings.Repeat(" ", width-visible)
}

func clamp(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
