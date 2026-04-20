package analyze

import (
	"testing"
	"time"
)

func TestStateStoreUnsubscribeLeavesChannelOpen(t *testing.T) {
	state := NewStateStore(Config{Endpoint: "http://ai", Model: "gpt-4o"})

	snapshots, unsubscribe := state.Subscribe(1)
	select {
	case <-snapshots:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for initial snapshot")
	}

	unsubscribe()

	done := make(chan struct{})
	go func() {
		state.Update(func(snapshot *SessionSnapshot) {
			snapshot.Status = "updated"
		})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for post-unsubscribe update")
	}

	select {
	case _, ok := <-snapshots:
		if !ok {
			t.Fatal("unsubscribe unexpectedly closed the subscriber channel")
		}
		t.Fatal("received unexpected snapshot after unsubscribe")
	default:
	}
}
