package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/erdajt/wraith/internal/runner"
	"github.com/erdajt/wraith/internal/session"
)

func TestCompletedEvidenceWaitsForDraftAndDoesNotCrossSessions(t *testing.T) {
	a := fixture(t)
	r := runner.Record{ID: session.ID(), Session: a.Session.ID, Agent: true, ShareOutput: true, Output: "evidence", Finished: time.Now()}
	a.pendingRuns[r.ID] = r
	a.Prompt.Set("unsent draft")
	a.deliverRuns()
	if len(a.Session.Messages) != 0 || a.Prompt.String() != "unsent draft" {
		t.Fatal("completion interrupted draft")
	}
	a.Prompt.Set("")
	original := a.Session
	a.Session = session.New("other")
	a.deliverRuns()
	if len(a.Session.Messages) != 0 {
		t.Fatal("output crossed session boundary")
	}
	a.Session = original
	a.agentStopped = true
	a.deliverRuns()
	a.pendingRuns[r.ID] = r
	a.deliverRuns()
	if len(a.Session.Messages) != 1 || len(a.Session.DeliveredRuns) != 1 || a.busy {
		t.Fatal("duplicate evidence or stopped turn resumed")
	}
	saved, err := a.Store.Load(original.ID)
	if err != nil || len(saved.DeliveredRuns) != 1 {
		t.Fatal("delivery not durable")
	}
}

func TestCompletionDoesNotInterruptEditorOrSelection(t *testing.T) {
	a := fixture(t)
	r := runner.Record{ID: session.ID(), Session: a.Session.ID, Agent: true, Finished: time.Now()}
	a.pendingRuns[r.ID] = r
	a.editing = true
	a.deliverRuns()
	a.editing = false
	a.showRun(r, "text")
	a.popup.Reader.Frozen = true
	a.deliverRuns()
	if len(a.Session.Messages) != 0 || len(a.pendingRuns) != 1 {
		t.Fatal("interactive barrier ignored")
	}
}

func TestHiddenResultDisclosesStatusWithoutOutputOrError(t *testing.T) {
	a := fixture(t)
	a.agentStopped = true
	r := runner.Record{ID: session.ID(), Session: a.Session.ID, Agent: true, ShareOutput: true, OutputHidden: true, Command: "printf result", Status: "failed", Error: "private-error", Output: "private-output", Finished: time.Now()}
	a.pendingRuns[r.ID] = r
	a.deliverRuns()
	got := a.Session.Messages[0].Content
	if strings.Contains(got, "private-") || !strings.Contains(got, "output withheld") || len(a.Session.SharedRuns) != 0 {
		t.Fatalf("hidden result: %s", got)
	}
}
