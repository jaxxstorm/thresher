package analyze

import (
	"fmt"
	"strings"
	"sync"
)

type SessionSnapshot struct {
	Endpoint        string   `json:"endpoint"`
	Status          string   `json:"status"`
	LastEvent       string   `json:"last_event"`
	Phase           string   `json:"phase"`
	Model           string   `json:"model"`
	Records         int      `json:"records"`
	TotalBytes      int      `json:"total_bytes"`
	PendingPackets  int      `json:"pending_packets"`
	PendingBytes    int      `json:"pending_bytes"`
	UploadedBatches int      `json:"uploaded_batches"`
	InFlight        bool     `json:"in_flight"`
	LimitReached    bool     `json:"limit_reached"`
	Paused          bool     `json:"paused"`
	Completed       bool     `json:"completed"`
	Error           string   `json:"error,omitempty"`
	BatchPackets    int      `json:"batch_packets"`
	BatchBytes      int      `json:"batch_bytes"`
	SessionPackets  int      `json:"session_packets"`
	SessionBytes    int      `json:"session_bytes"`
	Models          []string `json:"models,omitempty"`
	Analysis        []string `json:"analysis,omitempty"`
	Events          []string `json:"events,omitempty"`
}

type StateStore struct {
	mu          sync.RWMutex
	snapshot    SessionSnapshot
	subscribers map[chan SessionSnapshot]struct{}
}

func NewStateStore(config Config) *StateStore {
	return &StateStore{
		snapshot: SessionSnapshot{
			Endpoint:       config.Endpoint,
			Status:         "waiting for packets",
			LastEvent:      "session created",
			Phase:          "idle",
			Model:          config.Model,
			BatchPackets:   config.BatchPackets,
			BatchBytes:     config.BatchBytes,
			SessionPackets: config.SessionPackets,
			SessionBytes:   config.SessionBytes,
		},
		subscribers: make(map[chan SessionSnapshot]struct{}),
	}
}

func (s *StateStore) Snapshot() SessionSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneSessionSnapshot(s.snapshot)
}

func (s *StateStore) Subscribe(buffer int) (<-chan SessionSnapshot, func()) {
	if buffer <= 0 {
		buffer = 1
	}

	ch := make(chan SessionSnapshot, buffer)
	s.mu.Lock()
	s.subscribers[ch] = struct{}{}
	snapshot := cloneSessionSnapshot(s.snapshot)
	s.mu.Unlock()
	ch <- snapshot

	return ch, func() {
		s.mu.Lock()
		delete(s.subscribers, ch)
		close(ch)
		s.mu.Unlock()
	}
}

func (s *StateStore) Update(update func(*SessionSnapshot)) {
	s.mu.Lock()
	next := cloneSessionSnapshot(s.snapshot)
	update(&next)
	s.snapshot = next
	subscribers := make([]chan SessionSnapshot, 0, len(s.subscribers))
	for ch := range s.subscribers {
		subscribers = append(subscribers, ch)
	}
	s.mu.Unlock()

	for _, ch := range subscribers {
		publishSnapshot(ch, next)
	}
}

func (s *StateStore) ActiveModel() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot.Model
}

func (s *StateStore) IsPaused() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot.Paused
}

func (s *StateStore) SetActiveModel(model string) {
	model = strings.TrimSpace(model)
	if model == "" {
		return
	}

	s.Update(func(snapshot *SessionSnapshot) {
		snapshot.Model = model
		snapshot.Status = fmt.Sprintf("switched model to %s", model)
		snapshot.LastEvent = snapshot.Status
		pushSnapshotEvent(snapshot, snapshot.Status)
		if !snapshot.Paused && !snapshot.InFlight && !snapshot.LimitReached && !snapshot.Completed {
			snapshot.Phase = "collecting"
		}
	})
}

func (s *StateStore) SetPaused(paused bool) {
	s.Update(func(snapshot *SessionSnapshot) {
		snapshot.Paused = paused
		if paused {
			snapshot.Status = "analysis paused"
			snapshot.Phase = "paused"
		} else {
			snapshot.Status = "analysis resumed"
			switch {
			case snapshot.LimitReached:
				snapshot.Phase = "limited"
			case snapshot.InFlight:
				snapshot.Phase = "uploading"
			case snapshot.PendingPackets > 0 || snapshot.Records > 0:
				snapshot.Phase = "collecting"
			default:
				snapshot.Phase = "idle"
			}
		}
		snapshot.LastEvent = snapshot.Status
		pushSnapshotEvent(snapshot, snapshot.Status)
	})
}

func cloneSessionSnapshot(snapshot SessionSnapshot) SessionSnapshot {
	snapshot.Models = append([]string(nil), snapshot.Models...)
	snapshot.Analysis = append([]string(nil), snapshot.Analysis...)
	snapshot.Events = append([]string(nil), snapshot.Events...)
	return snapshot
}

func publishSnapshot(ch chan SessionSnapshot, snapshot SessionSnapshot) {
	snapshot = cloneSessionSnapshot(snapshot)
	select {
	case ch <- snapshot:
	default:
		select {
		case <-ch:
		default:
		}
		ch <- snapshot
	}
}

func pushSnapshotEvent(snapshot *SessionSnapshot, event string) {
	event = strings.TrimSpace(event)
	if event == "" {
		return
	}
	snapshot.Events = append(snapshot.Events, event)
	if len(snapshot.Events) > 8 {
		snapshot.Events = snapshot.Events[len(snapshot.Events)-8:]
	}
}

func markSessionComplete(snapshot *SessionSnapshot) {
	if snapshot.Error != "" || snapshot.Completed {
		return
	}

	snapshot.Completed = true
	if snapshot.LimitReached {
		snapshot.Status = "analysis limit reached"
		snapshot.Phase = "limited"
		snapshot.LastEvent = "analysis limit reached; stopping uploads"
		return
	}
	if snapshot.Paused {
		snapshot.Status = "analysis paused"
		snapshot.Phase = "paused"
		snapshot.LastEvent = snapshot.Status
		return
	}

	snapshot.Status = "analysis complete"
	snapshot.LastEvent = snapshot.Status
	snapshot.Phase = "complete"
	pushSnapshotEvent(snapshot, snapshot.Status)
}

func markSessionError(snapshot *SessionSnapshot, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	snapshot.Error = text
	snapshot.Status = text
	snapshot.LastEvent = text
	snapshot.Phase = "error"
	snapshot.InFlight = false
	pushSnapshotEvent(snapshot, text)
}
