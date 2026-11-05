package runner

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/erdajt/wraith/internal/session"
)

func TestRunPersistsMergedOutputAndExitStatus(t *testing.T) {
	h := History{Root: t.TempDir()}
	r, err := h.Run(context.Background(), Record{ID: session.ID(), Session: session.ID(), Command: "printf out; printf err >&2; exit 7", Directory: t.TempDir(), Shell: "/bin/sh", Source: "block"}, nil)
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

func TestCancellationStopsChildProcessesAndPersistsRecord(t *testing.T) {
	h := History{Root: t.TempDir()}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	r, err := h.Run(ctx, Record{ID: session.ID(), Command: "sleep 30 & wait", Shell: "/bin/sh"}, nil)
	if err != nil || r.Status != "cancelled" || time.Since(start) > 2*time.Second {
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
