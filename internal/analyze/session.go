package analyze

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jaxxstorm/thresher/internal/capture"
	"golang.org/x/term"
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
	client      *Client
	config      Config
	ui          *Model
	count       int
	bytes       int
	batch       []capture.Record
	models      []ModelInfo
	programOpts []tea.ProgramOption
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
	return &Session{client: client, config: config, ui: NewModel(config)}
}

func (s *Session) RunLive(ctx context.Context, open capture.StreamOpener) error {
	return s.runSession(ctx, func(runCtx context.Context, program *tea.Program) error {
		if models, err := s.client.ListModels(runCtx); err == nil {
			s.models = models
			program.Send(modelsLoadedMsg(models))
		}

		err := capture.StreamRecords(runCtx, open, func(record capture.Record) error {
			return s.consumeRecord(runCtx, program, record)
		})
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			program.Send(errorMsg{text: fmt.Sprintf("capture error: %v", err)})
			return err
		}
		return s.flush(runCtx, program)
	})
}

func (s *Session) RunReader(ctx context.Context, r io.Reader) error {
	return s.runSession(ctx, func(runCtx context.Context, program *tea.Program) error {
		if models, err := s.client.ListModels(runCtx); err == nil {
			s.models = models
			program.Send(modelsLoadedMsg(models))
		}

		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			if err := runCtx.Err(); err != nil {
				return nil
			}
			var record capture.Record
			if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
				program.Send(errorMsg{text: fmt.Sprintf("decoding analysis input record: %v", err)})
				return fmt.Errorf("decoding analysis input record: %w", err)
			}
			if err := s.consumeRecord(runCtx, program, record); err != nil {
				if err == context.Canceled {
					return nil
				}
				program.Send(errorMsg{text: err.Error()})
				return err
			}
		}
		if err := scanner.Err(); err != nil {
			program.Send(errorMsg{text: fmt.Sprintf("reading analysis input: %v", err)})
			return fmt.Errorf("reading analysis input: %w", err)
		}
		return s.flush(runCtx, program)
	})
}

func (s *Session) runSession(ctx context.Context, worker func(context.Context, *tea.Program) error) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	s.ui.SetCancel(cancel)
	program := s.startProgram(runCtx)
	errCh := make(chan error, 1)

	go func() {
		err := worker(runCtx, program)
		errCh <- err
		program.Quit()
	}()

	_, runErr := program.Run()
	if s.ui.QuitRequested() {
		cancel()
		return nil
	}
	if runErr != nil {
		if errors.Is(runErr, tea.ErrProgramKilled) && errors.Is(runCtx.Err(), context.Canceled) {
			return nil
		}
		cancel()
		return runErr
	}
	if errors.Is(runCtx.Err(), context.Canceled) {
		return nil
	}
	cancel()
	workerErr := <-errCh
	if errors.Is(workerErr, context.Canceled) {
		return nil
	}
	return workerErr
}

func (s *Session) startProgram(ctx context.Context) *tea.Program {
	options := []tea.ProgramOption{tea.WithContext(ctx)}
	if len(s.programOpts) > 0 {
		options = append(options, s.programOpts...)
	} else if isInteractiveSession() {
		options = append(options,
			tea.WithAltScreen(),
			tea.WithInput(os.Stdin),
			tea.WithOutput(os.Stdout),
		)
	} else {
		options = append(options, tea.WithInput(nil), tea.WithOutput(io.Discard))
	}

	return tea.NewProgram(s.ui, options...)
}

func (s *Session) consumeRecord(ctx context.Context, program *tea.Program, record capture.Record) error {
	encoded, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encoding record for analysis: %w", err)
	}
	program.Send(recordMsg{record: record, encodedBytes: len(encoded)})

	if s.count >= s.config.SessionPackets || s.bytes+len(encoded) > s.config.SessionBytes {
		program.Send(limitReachedMsg{reason: "analysis limit reached; stopping uploads"})
		return context.Canceled
	}

	s.batch = append(s.batch, record)
	s.count++
	s.bytes += len(encoded)

	batchBytes := s.batchSize()
	program.Send(batchQueuedMsg{pendingPackets: len(s.batch), pendingBytes: batchBytes})

	if s.ui.IsPaused() {
		program.Send(statusMsg{text: fmt.Sprintf("analysis paused with %d buffered packets", len(s.batch))})
		return nil
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
	if s.ui.IsPaused() {
		program.Send(statusMsg{text: fmt.Sprintf("analysis paused with %d buffered packets", len(s.batch))})
		return nil
	}

	batchPackets := len(s.batch)
	batchBytes := s.batchSize()
	activeModel := s.ui.ActiveModel()
	if activeModel == "" {
		activeModel = s.config.Model
	}

	program.Send(uploadStartedMsg{batchPackets: batchPackets, batchBytes: batchBytes})
	resp, err := s.client.Analyze(ctx, AnalyzeRequest{
		Model:     activeModel,
		System:    "You are analyzing decoded Tailscale packet capture output. Explain what is happening, identify notable flows, failures, or unusual behavior, and be concise but informative.",
		Prompt:    buildBatchPrompt(s.batch),
		MaxTokens: s.config.MaxTokens,
	})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil
		}
		program.Send(errorMsg{text: fmt.Sprintf("analysis request failed: %v", err)})
		return err
	}
	program.Send(analysisMsg{text: resp.Text, batchPackets: batchPackets, batchBytes: batchBytes})
	s.batch = nil
	return nil
}

func (s *Session) batchSize() int {
	batchBytes := 0
	for _, item := range s.batch {
		payload, _ := json.Marshal(item)
		batchBytes += len(payload)
	}
	return batchBytes
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

func isInteractiveSession() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}
