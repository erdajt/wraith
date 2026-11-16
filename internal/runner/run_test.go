package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/erdajt/wraith/internal/session"
)

func TestRunPersistsMergedOutputAndExitStatus(t *testing.T) {
	h := History{Root: t.TempDir()}
	r, err := h.Run(t.Context(), Record{ID: session.ID(), Session: session.ID(), Command: "printf out; printf err >&2; exit 7", Directory: t.TempDir(), Shell: "/bin/sh", Source: "block"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := h.Load(r.ID)
	if err != nil || saved.Output != "outerr" || saved.ExitCode != 7 || saved.Status != "failed" {
		t.Fatalf("bad run: %+v %v", saved, err)
	}
	list, err := h.List()
	if err != nil || len(list) != 1 || list[0].Command != r.Command {
		t.Fatalf("history missing: %v", err)
	}
}

func TestDeadlineStopsChildProcessesAndPersistsRecord(t *testing.T) {
	h := History{Root: t.TempDir()}
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	r, err := h.Run(ctx, Record{ID: session.ID(), Command: "sleep 30 & wait", Shell: "/bin/sh"}, nil)
	if err != nil || r.Status != "timed_out" || time.Since(start) > 2*time.Second {
		t.Fatalf("cancel: %+v %v", r, err)
	}
}

func TestCaptureBoundsOutputAndRejectsTraversal(t *testing.T) {
	c := &capture{}
	input := strings.Repeat("x", OutputLimit+100)
	n, err := c.Write([]byte(input))
	if err != nil || n != len(input) || c.text.Len() != OutputLimit || !c.truncated {
		t.Fatal("capture limit failed")
	}
	if _, err := (History{Root: t.TempDir()}).Load("../escape"); err == nil {
		t.Fatal("accepted traversal")
	}
}

func TestExplicitCancellationRetainsPartialOutput(t *testing.T) {
	t.Parallel()
	h := History{Root: t.TempDir()}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	r, err := h.Run(ctx, Record{ID: session.ID(), Command: "printf ready; sleep 30 & wait", Shell: "/bin/sh"}, func(string) { cancel() })
	if err != nil || r.Status != "cancelled" || r.Output != "ready" {
		t.Fatalf("cancel lost output or status: %+v %v", r, err)
	}
}

func TestShellExitTerminatesRemainingProcessGroup(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	h := History{Root: t.TempDir()}
	record, err := h.Run(t.Context(), Record{ID: session.ID(), Shell: "/bin/sh", Directory: directory, Command: "sleep 30 </dev/null >/dev/null 2>&1 & printf '%s' $!"}, nil)
	if err != nil || record.Status != "completed" {
		t.Fatalf("run: %+v %v", record, err)
	}
	var pid int
	if _, err := fmt.Sscan(record.Output, &pid); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		state, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
		if os.IsNotExist(err) || strings.Contains(string(state), ") Z ") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("child survived shell exit")
}

func TestRunDoesNotOverwriteCorruptHistory(t *testing.T) {
	t.Parallel()
	h := History{Root: t.TempDir()}
	id := session.ID()
	path := filepath.Join(h.Root, id+".json.gz")
	if err := os.WriteFile(path, []byte("evidence"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Run(t.Context(), Record{ID: id, Shell: "/bin/sh", Command: "true"}, nil); err == nil {
		t.Fatal("overwrote history")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "evidence" {
		t.Fatalf("history lost: %q %v", data, err)
	}
}
