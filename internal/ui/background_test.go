package ui

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/erdajt/wraith/internal/contextual"
	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/runner"
)

func TestAgentRunLeavesDraftAndChatAvailable(t *testing.T) {
	a := fixture(t)
	a.Prompt.Set("work on something else")
	a.approve(proposal{Command: "printf 'live marker\\n'; sleep 30", Continue: true})
	id := a.proposal.launch.ID
	a.runApproved()
	if a.executing || a.busy || a.popup != nil || a.Prompt.String() != "work on something else" {
		t.Fatal("background launch blocked the chat or replaced its draft")
	}
	saved, err := a.Store.Load(a.Session.ID)
	if err != nil || len(saved.Messages) != 1 || saved.Messages[0].Evidence == nil || saved.Messages[0].Evidence.ID != id {
		t.Fatalf("missing durable job receipt: %v", err)
	}
	calls := 0
	a.Client.HTTP.Transport = transport(func(req *http.Request) (*http.Response, error) {
		var body ollama.Request
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		calls++
		last := body.Messages[len(body.Messages)-1].Content
		if calls == 1 && last != "work on something else" {
			t.Errorf("new prompt lost: %s", last)
		}
		if calls == 2 && (!strings.Contains(last, "live marker") || !strings.Contains(last, "status: running") || !strings.Contains(last, id)) {
			t.Errorf("live output query lost job identity or status: %s", last)
		}
		return response(`{"done":true,"message":{"role":"assistant","content":"received"}}`), nil
	})
	a.submit()
	drain(t, a)
	deadline := time.After(3 * time.Second)
	for {
		r, err := a.jobRecord(id)
		if err != nil || !r.Background || !r.Finished.IsZero() {
			t.Fatalf("job stopped while chatting: %+v %v", r, err)
		}
		if strings.Contains(r.Output, "live marker") {
			break
		}
		select {
		case <-a.jobs.Updates:
		case <-deadline:
			t.Fatal("job did not produce live output")
		}
	}
	a.pendingTool = &contextual.ToolRequest{Tool: "output", ID: id, Query: runner.Query{Pattern: "^live", Regex: true, Limit: 1}}
	a.runPendingTool()
	drain(t, a)
	if calls != 2 || a.executing || a.popup != nil {
		t.Fatalf("output query blocked chat: calls=%d", calls)
	}
}

func finishJobs(t *testing.T, a *App) {
	t.Helper()
	deadline := time.After(3 * time.Second)
	polling := time.NewTicker(5 * time.Millisecond)
	defer polling.Stop()
	for a.busy || len(a.jobs.Snapshot()) > 0 {
		select {
		case now := <-polling.C:
			a.Update(tick(now))
		case msg := <-a.events:
			a.Update(msg)
		case <-deadline:
			t.Fatal("jobs did not finish")
		}
	}
}

func TestBackgroundLaunchFailuresKeepChatAndHistoryAvailable(t *testing.T) {
	for _, failure := range []string{"process", "receipt"} {
		t.Run(failure, func(t *testing.T) {
			a := fixture(t)
			a.Prompt.Set("retain draft")
			if failure == "process" {
				a.Config.Shell = []string{filepath.Join(t.TempDir(), "missing-shell")}
			}
			a.approve(proposal{Command: "printf evidence", Continue: true})
			id := a.proposal.launch.ID
			if failure == "receipt" {
				a.Store.Root = filepath.Join(t.TempDir(), "blocked")
				if err := os.WriteFile(a.Store.Root, []byte("blocked"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			a.runApproved()
			if a.busy || a.executing || a.popup != nil || a.Prompt.String() != "retain draft" {
				t.Fatal("launch failure took over chat")
			}
			if failure == "receipt" && !strings.Contains(a.notice, "receipt save failed") {
				t.Fatalf("receipt failure not reported: %s", a.notice)
			}
			a.runApproved()
			finishJobs(t, a)
			records, err := a.history().List()
			if err != nil || len(records) != 1 || records[0].ID != id || records[0].Finished.IsZero() {
				t.Fatalf("job lost or relaunched: %+v %v", records, err)
			}
			r, err := a.jobRecord(id)
			if err != nil {
				t.Fatal(err)
			}
			if failure == "process" && (r.Status != "failed" || r.Error == "") {
				t.Fatalf("startup failure not retained: %+v", r)
			}
			if failure == "receipt" && (r.Status != "completed" || r.Output != "evidence") {
				t.Fatalf("receipt failure lost command output: %+v", r)
			}
		})
	}
}

func TestBackgroundReceiptRespectsInteractiveBarriers(t *testing.T) {
	for _, barrier := range []string{"editor", "shell", "selection"} {
		t.Run(barrier, func(t *testing.T) {
			a := fixture(t)
			a.editing = barrier == "editor"
			if barrier == "shell" {
				a.focus = "terminal0"
			}
			if barrier == "selection" {
				a.visualMode = "visual"
			}
			a.approve(proposal{Command: "sleep 30", Continue: true})
			a.runApproved()
			if a.busy || a.executing || a.popup != nil || len(a.Session.Messages) != 1 {
				t.Fatal("background receipt interrupted an interactive action")
			}
		})
	}
}
