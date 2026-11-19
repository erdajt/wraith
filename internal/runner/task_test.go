package runner

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/erdajt/wraith/internal/session"
)

func TestExtensionTaskUsesDurableBoundedJobHistory(t *testing.T) {
	t.Parallel()
	history := History{Root: t.TempDir()}
	manager := NewManager(history, 1)
	t.Cleanup(manager.Close)
	id := session.ID()
	data := strings.Repeat("output\n", 20000)
	if err := manager.StartTask(t.Context(), Record{ID: id, Command: "extension.inspect", Source: "extension", CaptureLimit: 64 << 10, TailLimit: 4 << 10}, time.Second, func(ctx context.Context, output io.Writer) error {
		if _, err := history.Load(id); err != nil {
			return err
		}
		_, err := io.WriteString(output, data)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	for {
		job := manager.Snapshot()[0]
		if !job.Record.Finished.IsZero() {
			if job.Err != nil || job.Record.Status != "completed" || !job.Record.Truncated || len(job.Record.Output) != 64<<10 || len(job.Record.Tail) != 4<<10 || job.Record.Observed != int64(len(data)) {
				t.Fatalf("task capture: %+v", job)
			}
			saved, err := history.Load(id)
			if err != nil || saved.Output != data[:64<<10] || saved.ExitCode != 0 || saved.Source != "extension" {
				t.Fatalf("task persistence: %+v, %v", saved, err)
			}
			break
		}
		select {
		case <-manager.Updates:
		case <-deadline.C:
			t.Fatal("task did not finish")
		}
	}
}

func TestExtensionTaskCancellationAndFailureAreRecorded(t *testing.T) {
	for _, mode := range []string{"cancelled", "timed_out", "failed"} {
		t.Run(mode, func(t *testing.T) {
			manager := NewManager(History{Root: t.TempDir()}, 1)
			t.Cleanup(manager.Close)
			id := session.ID()
			timeout := time.Second
			if mode == "timed_out" {
				timeout = 5 * time.Millisecond
			}
			err := manager.StartTask(t.Context(), Record{ID: id, Command: "extension.inspect", Session: "s", Generation: "g", AgentRequested: true}, timeout, func(ctx context.Context, output io.Writer) error {
				if mode == "failed" {
					return errors.New("tool rejected input")
				}
				<-ctx.Done()
				return ctx.Err()
			})
			if err != nil {
				t.Fatal(err)
			}
			if mode == "cancelled" {
				manager.CancelTurn("s", "g")
			}
			deadline := time.NewTimer(3 * time.Second)
			defer deadline.Stop()
			for {
				job := manager.Snapshot()[0]
				if !job.Record.Finished.IsZero() {
					if job.Record.Status != mode || job.Err != nil || job.Record.Error == "" || job.Record.ExitCode != 1 {
						t.Fatalf("task status: %+v", job)
					}
					return
				}
				select {
				case <-manager.Updates:
				case <-deadline.C:
					t.Fatal("task did not stop")
				}
			}
		})
	}
}
