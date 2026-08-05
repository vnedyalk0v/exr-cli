package agent

import (
	"context"
	"testing"
	"time"

	"github.com/vnedyalk0v/exr-cli/internal/session"
)

func TestDemoTurnApprovePath(t *testing.T) {
	sess := session.New("test")
	d := &DemoRunner{Sess: sess}
	ev := make(chan Event, 16)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go d.RunTurn(ctx, "fix the prompt", ev)

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
					// may have been approved already
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
