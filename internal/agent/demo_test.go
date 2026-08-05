package agent

import (
	"context"
	"testing"
	"time"

	"github.com/vnedyalk0v/exr-cli/internal/session"
)

func TestDemoTurnApprovePath(t *testing.T) {
	sess := session.New("test")
	r := &Runner{Sess: sess} // Client nil → demo
	ev := make(chan Event, 16)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go r.RunTurn(ctx, "fix the prompt", ev)

	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for turn")
		case e, ok := <-ev:
			if !ok {
				t.Fatal("channel closed early")
			}
			if e.Kind == "blocked" {
				idx := sess.FirstBlocked()
				if idx < 0 {
					continue
				}
				ApproveStep(sess, idx)
			}
			if e.Kind == "done" {
				meta, steps := sess.Snapshot()
				if !meta.Synthetic {
					t.Error("expected synthetic session")
				}
				if len(steps) < 3 {
					t.Fatalf("expected several steps, got %d", len(steps))
				}
				return
			}
		}
	}
}
