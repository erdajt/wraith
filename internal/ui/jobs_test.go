package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/runner"
	"github.com/erdajt/wraith/internal/session"
)

func TestRunningJobIsBrowsableAndCancellable(t *testing.T) {
	a := fixture(t)
	a.approve(proposal{Command: "printf live; sleep 30"})
	a.runApproved()
	a.runCommand("jobs")
	if a.popup == nil || a.popup.Kind != "jobs" || len(a.popup.Items) != 1 {
		t.Fatalf("jobs unavailable during execution: %s", a.notice)
	}
	a.menuKey(tea.KeyMsg{Type: tea.KeyEnter})
	if a.popup.Kind != "run-output" {
		t.Fatal("live reader did not open")
	}
	a.readerKey(key(a.Config.Keys["job_cancel"]))
	drain(t, a)
	if a.activeRun.Status != "cancelled" {
		t.Fatalf("job did not cancel: %+v", a.activeRun)
	}
	a.openJobs()
	if !strings.Contains(a.popup.Items[0].Detail, "cancelled") {
		t.Fatal("job status not retained")
	}
}

func TestJobsPickerFiltersSessionButHistoryRetainsCommands(t *testing.T) {
	a := fixture(t)
	for _, id := range []string{a.Session.ID, session.ID()} {
		if err := a.history().Save(runner.Record{ID: session.ID(), Session: id, Command: "printf x", Finished: time.Now(), Status: "completed"}); err != nil {
			t.Fatal(err)
		}
	}
	a.openJobs()
	if len(a.popup.Items) != 1 {
		t.Fatal("jobs mixed sessions")
	}
	a.openHistory()
	if len(a.popup.Items) != 2 {
		t.Fatal("history lost commands")
	}
}
