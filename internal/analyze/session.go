package analyze

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jaxxstorm/thresher/internal/capture"
)

type Config struct {
	Endpoint       string
	Model          string
	EndpointStyle  EndpointStyle
	BatchPackets   int
	BatchBytes     int
	SessionPackets int
	SessionBytes   int
	MaxTokens      int
}

type Session struct {
	client *Client
	config Config
	ui     *Model
	count  int
	bytes  int
	batch  []capture.Record
	models []ModelInfo
}

func NewSession(config Config) *Session {
	if config.BatchPackets <= 0 {
		config.BatchPackets = 20
	}
	if config.BatchBytes <= 0 {
		config.BatchBytes = 64 * 1024
	}
	if config.SessionPackets <= 0 {
		config.SessionPackets = 500
	}
	if config.SessionBytes <= 0 {
		config.SessionBytes = 2 * 1024 * 1024
	}
	if config.MaxTokens <= 0 {
		config.MaxTokens = 300
	}

	client := NewClient(config.Endpoint, config.EndpointStyle)
	ui := NewModel(config)
	return &Session{client: client, config: config, ui: &ui}
}

func (s *Session) RunLive(ctx context.Context, open capture.StreamOpener) error {
	program := s.startProgram(ctx)

	if models, err := s.client.ListModels(ctx); err == nil {
		s.models = models
		program.Send(modelsLoadedMsg(models))
	}

	err := capture.StreamRecords(ctx, open, func(record capture.Record) error {
		program.Send(recordMsg(record))
		return s.consumeRecord(ctx, program, record)
	})
	if err != nil {
		program.Send(statusMsg(fmt.Sprintf("capture error: %v", err)))
		return err
	}
	return s.flush(ctx, program)
}

func (s *Session) RunReader(ctx context.Context, r io.Reader) error {
	program := s.startProgram(ctx)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		var record capture.Record
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return fmt.Errorf("decoding analysis input record: %w", err)
		}
		program.Send(recordMsg(record))
		if err := s.consumeRecord(ctx, program, record); err != nil {
			if err == context.Canceled {
				return nil
			}
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading analysis input: %w", err)
	}
	return s.flush(ctx, program)
}

func (s *Session) startProgram(ctx context.Context) *tea.Program {
	program := tea.NewProgram(s.ui, tea.WithContext(ctx), tea.WithInput(nil))
	go func() {
		_, _ = program.Run()
	}()
	if models, err := s.client.ListModels(ctx); err == nil {
		s.models = models
		program.Send(modelsLoadedMsg(models))
	}
	return program
}

func (s *Session) consumeRecord(ctx context.Context, program *tea.Program, record capture.Record) error {
	encoded, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encoding record for analysis: %w", err)
	}
	if s.count >= s.config.SessionPackets || s.bytes+len(encoded) > s.config.SessionBytes {
		program.Send(statusMsg("analysis limit reached; stopping uploads"))
		return context.Canceled
	}

	s.batch = append(s.batch, record)
	s.count++
	s.bytes += len(encoded)

	batchBytes := 0
	for _, item := range s.batch {
		payload, _ := json.Marshal(item)
		batchBytes += len(payload)
	}

	if len(s.batch) >= s.config.BatchPackets || batchBytes >= s.config.BatchBytes {
		return s.flush(ctx, program)
	}
	return nil
}

func (s *Session) flush(ctx context.Context, program *tea.Program) error {
	if len(s.batch) == 0 {
		return nil
	}
	program.Send(statusMsg(fmt.Sprintf("uploading batch of %d packets", len(s.batch))))
	prompt := buildBatchPrompt(s.batch)
	resp, err := s.client.Analyze(ctx, AnalyzeRequest{
		Model:     s.config.Model,
		System:    "You are analyzing decoded Tailscale packet capture output. Explain what is happening, identify notable flows, failures, or unusual behavior, and be concise but informative.",
		Prompt:    prompt,
		MaxTokens: s.config.MaxTokens,
	})
	if err != nil {
		return err
	}
	program.Send(analysisMsg(resp.Text))
	s.batch = nil
	return nil
}

func buildBatchPrompt(records []capture.Record) string {
	lines := make([]string, 0, len(records)+2)
	lines = append(lines, "Analyze this packet capture window and explain what is happening:")
	for _, record := range records {
		line := fmt.Sprintf("%d %s %s -> %s %s %s", record.Number, record.Time, record.Src, record.Dst, record.Protocol, record.Info)
		if record.StreamID != "" {
			line += fmt.Sprintf(" stream=%s", record.StreamID)
		}
		if record.Analysis != nil && len(record.Analysis.Annotations) > 0 {
			line += fmt.Sprintf(" annotations=%s", strings.Join(record.Analysis.Annotations, ","))
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
