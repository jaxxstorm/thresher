package analyze

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jaxxstorm/thresher/internal/capture"
)

type Config struct {
	Endpoint       string
	Model          string
	UserAgent      string
	WebAccess      WebAccess
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
	state       *StateStore
	count       int
	bytes       int
	batch       []capture.Record
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

	client := NewClient(config.Endpoint, config.EndpointStyle, config.UserAgent)
	return &Session{client: client, config: config, state: NewStateStore(config)}
}

func (s *Session) State() *StateStore {
	return s.state
}

func (s *Session) RunLive(ctx context.Context, open capture.StreamOpener) error {
	return s.RunLiveWithPresenter(ctx, open, NewConsolePresenter(s.config, s.programOpts...))
}

func (s *Session) RunLiveWithPresenter(ctx context.Context, open capture.StreamOpener, presenter Presenter) error {
	return presenter.Run(ctx, s.state, func(runCtx context.Context) error {
		s.loadModels(runCtx)

		err := capture.StreamRecords(runCtx, open, func(record capture.Record) error {
			return s.consumeRecord(runCtx, record)
		})
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			text := fmt.Sprintf("capture error: %v", err)
			s.state.Update(func(snapshot *SessionSnapshot) {
				markSessionError(snapshot, text)
			})
			return err
		}
		return s.flush(runCtx)
	})
}

func (s *Session) RunReader(ctx context.Context, r io.Reader) error {
	return s.RunReaderWithPresenter(ctx, r, NewConsolePresenter(s.config, s.programOpts...))
}

func (s *Session) RunReaderWithPresenter(ctx context.Context, r io.Reader, presenter Presenter) error {
	return presenter.Run(ctx, s.state, func(runCtx context.Context) error {
		s.loadModels(runCtx)

		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			if err := runCtx.Err(); err != nil {
				return nil
			}

			var record capture.Record
			if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
				text := fmt.Sprintf("decoding analysis input record: %v", err)
				s.state.Update(func(snapshot *SessionSnapshot) {
					markSessionError(snapshot, text)
				})
				return fmt.Errorf("decoding analysis input record: %w", err)
			}
			if err := s.consumeRecord(runCtx, record); err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}
				return err
			}
		}
		if err := scanner.Err(); err != nil {
			text := fmt.Sprintf("reading analysis input: %v", err)
			s.state.Update(func(snapshot *SessionSnapshot) {
				markSessionError(snapshot, text)
			})
			return fmt.Errorf("reading analysis input: %w", err)
		}
		return s.flush(runCtx)
	})
}

func (s *Session) loadModels(ctx context.Context) {
	models, err := s.client.ListModels(ctx)
	if err != nil {
		return
	}

	ids := make([]string, 0, len(models))
	for _, model := range models {
		ids = append(ids, model.ID)
	}

	s.state.Update(func(snapshot *SessionSnapshot) {
		snapshot.Models = ids
		if len(ids) > 0 {
			snapshot.LastEvent = fmt.Sprintf("loaded %d models", len(ids))
			pushSnapshotEvent(snapshot, snapshot.LastEvent)
		}
	})
}

func (s *Session) consumeRecord(ctx context.Context, record capture.Record) error {
	encoded, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encoding record for analysis: %w", err)
	}

	encodedBytes := len(encoded)
	s.state.Update(func(snapshot *SessionSnapshot) {
		snapshot.Records++
		snapshot.TotalBytes += encodedBytes
		snapshot.Status = fmt.Sprintf("processed %d packets", snapshot.Records)
		snapshot.LastEvent = fmt.Sprintf("captured packet %d %s -> %s", snapshot.Records, record.Src, record.Dst)
		if !snapshot.Paused && !snapshot.InFlight && !snapshot.LimitReached && !snapshot.Completed {
			snapshot.Phase = "collecting"
		}
	})

	if s.count >= s.config.SessionPackets || s.bytes+encodedBytes > s.config.SessionBytes {
		s.state.Update(func(snapshot *SessionSnapshot) {
			snapshot.LimitReached = true
			snapshot.InFlight = false
			snapshot.Status = "analysis limit reached"
			snapshot.LastEvent = "analysis limit reached; stopping uploads"
			snapshot.Phase = "limited"
			pushSnapshotEvent(snapshot, snapshot.LastEvent)
		})
		return context.Canceled
	}

	s.batch = append(s.batch, record)
	s.count++
	s.bytes += encodedBytes

	batchBytes := s.batchSize()
	s.state.Update(func(snapshot *SessionSnapshot) {
		snapshot.PendingPackets = len(s.batch)
		snapshot.PendingBytes = batchBytes
		if !snapshot.Paused && !snapshot.InFlight && !snapshot.LimitReached && !snapshot.Completed {
			snapshot.Phase = "collecting"
		}
		snapshot.Status = fmt.Sprintf("buffering %d packets for the next upload", len(s.batch))
		snapshot.LastEvent = snapshot.Status
	})

	if s.state.IsPaused() {
		s.state.Update(func(snapshot *SessionSnapshot) {
			snapshot.Status = fmt.Sprintf("analysis paused with %d buffered packets", len(s.batch))
			snapshot.LastEvent = snapshot.Status
			snapshot.Phase = "paused"
		})
		return nil
	}

	if len(s.batch) >= s.config.BatchPackets || batchBytes >= s.config.BatchBytes {
		return s.flush(ctx)
	}
	return nil
}

func (s *Session) flush(ctx context.Context) error {
	if len(s.batch) == 0 {
		return nil
	}
	if s.state.IsPaused() {
		s.state.Update(func(snapshot *SessionSnapshot) {
			snapshot.Status = fmt.Sprintf("analysis paused with %d buffered packets", len(s.batch))
			snapshot.LastEvent = snapshot.Status
			snapshot.Phase = "paused"
		})
		return nil
	}

	batchPackets := len(s.batch)
	batchBytes := s.batchSize()
	activeModel := s.state.ActiveModel()
	if activeModel == "" {
		activeModel = s.config.Model
	}

	s.state.Update(func(snapshot *SessionSnapshot) {
		snapshot.PendingPackets = batchPackets
		snapshot.PendingBytes = batchBytes
		snapshot.InFlight = true
		snapshot.Phase = "uploading"
		snapshot.Status = fmt.Sprintf("uploading batch of %d packets", batchPackets)
		snapshot.LastEvent = snapshot.Status
		pushSnapshotEvent(snapshot, snapshot.Status)
	})

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
		text := fmt.Sprintf("analysis request failed: %v", err)
		s.state.Update(func(snapshot *SessionSnapshot) {
			markSessionError(snapshot, text)
		})
		return err
	}

	s.state.Update(func(snapshot *SessionSnapshot) {
		snapshot.Analysis = append(snapshot.Analysis, strings.TrimSpace(resp.Text))
		snapshot.PendingPackets = 0
		snapshot.PendingBytes = 0
		snapshot.InFlight = false
		snapshot.UploadedBatches++
		snapshot.Status = "analysis updated"
		snapshot.LastEvent = fmt.Sprintf("received analysis for %d packets", batchPackets)
		pushSnapshotEvent(snapshot, snapshot.LastEvent)
		switch {
		case snapshot.LimitReached:
			snapshot.Phase = "limited"
		case snapshot.Paused:
			snapshot.Phase = "paused"
		default:
			snapshot.Phase = "collecting"
		}
	})
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
		if record.PathID != 0 {
			line += fmt.Sprintf(" path_id=%d", record.PathID)
		}
		if record.SNAT != "" {
			line += fmt.Sprintf(" snat=%s", record.SNAT)
		}
		if record.DNAT != "" {
			line += fmt.Sprintf(" dnat=%s", record.DNAT)
		}
		if record.PayloadPreview != "" {
			line += fmt.Sprintf(" payload=%q", record.PayloadPreview)
		}
		if record.Analysis != nil && len(record.Analysis.Annotations) > 0 {
			line += fmt.Sprintf(" annotations=%s", strings.Join(record.Analysis.Annotations, ","))
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
