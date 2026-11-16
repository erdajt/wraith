package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/erdajt/wraith/internal/session"
)

func TestManagerCoalescesOutputWithoutBlockingCompletion(t *testing.T) {
	t.Parallel()
	m := NewManager(History{Root: t.TempDir()}, 2)
	t.Cleanup(m.Close)
	first, second := session.ID(), session.ID()
	for _, id := range []string{first, second} {
		if err := m.Start(t.Context(), Record{ID: id, Shell: "/bin/sh", Command: "i=0; while [ $i -lt 1000 ]; do printf line; i=$((i+1)); done"}, time.Second*5); err != nil {
			t.Fatal(err)
		}
	}
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	for {
		completed := 0
		for _, job := range m.Snapshot() {
			if job.Record.Status == "completed" {
				completed++
				if len(job.Record.Output) != 4000 || job.Err != nil {
					t.Fatalf("lost output: %+v", job)
				}
			}
		}
		if completed == 2 {
			break
		}
		select {
		case <-m.Updates:
		case <-deadline.C:
			t.Fatal("jobs stalled")
		}
	}
	m.Forget(first)
	if len(m.Snapshot()) != 1 {
		t.Fatal("finished job retained after delivery")
	}
}

func TestManagerCancellationIsScopedToOriginatingTurn(t *testing.T) {
	t.Parallel()
	m := NewManager(History{Root: t.TempDir()}, 2)
	t.Cleanup(m.Close)
	original, other := session.ID(), session.ID()
	for _, record := range []Record{
		{ID: original, Session: "first", Generation: "turn", Shell: "/bin/sh", Command: "sleep 30"},
		{ID: other, Session: "second", Generation: "turn", Shell: "/bin/sh", Command: "sleep 30"},
	} {
		if err := m.Start(t.Context(), record, time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.Start(t.Context(), Record{ID: session.ID(), Shell: "/bin/sh", Command: "true"}, time.Second); err == nil {
		t.Fatal("active job limit ignored")
	}
	m.CancelTurn("first", "turn")
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		finished := false
		for _, job := range m.Snapshot() {
			if job.Record.ID == original && job.Record.Status == "cancelled" {
				finished = true
			}
			if job.Record.ID == other && !job.Record.Finished.IsZero() {
				t.Fatal("unrelated session cancelled")
			}
		}
		if finished {
			break
		}
		select {
		case <-m.Updates:
		case <-deadline.C:
			t.Fatal("cancel did not finish")
		}
	}
	if !m.Cancel(other) {
		t.Fatal("remaining job disappeared")
	}
}

func TestManagerRefusesLaunchWithoutDurableIntent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "blocked")
	if err := os.WriteFile(path, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	m := NewManager(History{Root: path}, 1)
	t.Cleanup(m.Close)
	if err := m.Start(t.Context(), Record{ID: session.ID(), Shell: "/bin/sh", Command: "touch marker", Directory: root}, time.Second); err == nil {
		t.Fatal("launch succeeded without history")
	}
	if _, err := os.Stat(filepath.Join(root, "marker")); !os.IsNotExist(err) {
		t.Fatal("command ran after persistence failure")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := m.Start(ctx, Record{ID: session.ID(), Shell: "/bin/sh", Command: "true"}, time.Second); err == nil {
		t.Fatal("cancelled launch accepted")
	}
}
