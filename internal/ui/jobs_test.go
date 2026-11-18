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
	drain(t, a)
	a.runCommand("jobs")

	if a.popup == nil || a.popup.Kind != "jobs" || len(a.popup.Items) != 1 {
		t.Fatalf("jobs unavailable during execution: %s", a.notice)
	}

	a.menuKey(tea.KeyMsg{Type: tea.KeyEnter})

	if a.popup.Kind != "run-output" {
		t.Fatal("live reader did not open")
	}

	a.readerKey(key(a.Config.Keys["job_cancel"]))
	finishJobs(t, a)

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

func TestOutputReaderShowsActualTailAndPreservesFrozenSelection(t *testing.T) {
	a := fixture(t)
	r := runner.Record{ID: session.ID(), Output: "old prefix", Tail: "distinctive final line", TailOffset: 100000, Truncated: true, Finished: time.Now(), Status: "completed"}
	a.showRun(r, r.Output)

	if !strings.Contains(a.popup.Reader.Buffer.String(), "distinctive final line") || !strings.Contains(a.popup.Reader.Buffer.String(), "byte 100000") {
		t.Fatal("retained tail missing from history")
	}

	original := a.popup.Reader.Buffer.String()
	a.popup.Reader.Frozen = true
	r.Tail = "later output"
	a.showRun(r, r.Output)

	if a.popup.Reader.Buffer.String() != original {
		t.Fatal("live update overwrote selected output")
	}

	a.readerKey(key("r"))

	if !strings.Contains(a.popup.Reader.Buffer.String(), "later output") {
		t.Fatal("refresh did not reveal latest tail")
	}
}
