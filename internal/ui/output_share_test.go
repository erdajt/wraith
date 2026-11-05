package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/erdajt/wraith/internal/runner"
)

func TestOutputStaysPrivateUntilExplicitSharing(t *testing.T) {
	a := fixture(t)
	a.approve(proposal{Command: "printf private-evidence", Continue: true})
	a.runApproved()
	drain(t, a)
	if len(a.Session.Messages) != 0 {
		t.Fatal("default execution shared output")
	}
	if !strings.Contains(a.View(), "share to agent") {
		t.Fatal("share binding missing")
	}
	a.loading = true
	a.menuKey(key(a.Config.Keys["output_share"]))
	if len(a.Session.Messages) != 1 || !strings.Contains(a.Session.Messages[0].Content, "private-evidence") {
		t.Fatal("explicit share lost output")
	}
	a.shareRun(a.activeRun)
	if len(a.Session.Messages) != 1 {
		t.Fatal("output was shared twice")
	}
	saved, err := a.Store.Load(a.Session.ID)
	if err != nil || len(saved.SharedRuns) != 1 {
		t.Fatal("share state not persisted")
	}
}

func TestAutomaticSharingHidesManualAction(t *testing.T) {
	a := fixture(t)
	a.Config.Execution.AutoShare = true
	a.showRun(runner.Record{ID: "record", Finished: time.Now()}, "output")
	if strings.Contains(a.View(), "share to agent") {
		t.Fatal("automatic sharing still advertises manual action")
	}
}
