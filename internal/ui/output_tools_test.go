package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/erdajt/wraith/internal/contextual"
	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/runner"
	"github.com/erdajt/wraith/internal/session"
)

func TestOutputToolsRespectPrivacyAndSessionOwnership(t *testing.T) {
	for _, policy := range []string{"shared", "hidden", "other session"} {
		t.Run(policy, func(t *testing.T) {
			a := fixture(t)
			a.agentStopped = false
			a.environmentCommand("set TARGET=private.example", false)
			a.popup = nil
			r := runner.Record{ID: session.ID(), Session: a.Session.ID, ShareOutput: true, Output: "private.example failed\n", Status: "completed", Finished: time.Now()}
			if policy == "hidden" {
				r.OutputHidden = true
			}
			if policy == "other session" {
				r.Session = session.ID()
			}
			if err := a.history().Save(r); err != nil {
				t.Fatal(err)
			}
			a.Client.HTTP.Transport = transport(func(req *http.Request) (*http.Response, error) {
				var body ollama.Request
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				text := body.Messages[len(body.Messages)-1].Content
				if strings.Contains(text, "private.example") {
					t.Error("private literal reached provider")
				}
				if policy == "shared" && (!strings.Contains(text, "wraith::TARGET failed") || !strings.Contains(text, `"line":1`)) {
					t.Errorf("missing searched evidence: %s", text)
				}
				if policy != "shared" && (!strings.Contains(text, "denied") || strings.Contains(text, "failed")) {
					t.Errorf("unauthorized evidence: %s", text)
				}
				return response(`{"done":true,"message":{"role":"assistant","content":"checked"}}`), nil
			})
			a.pendingTool = &contextual.ToolRequest{Tool: "output", ID: r.ID, Query: runner.Query{Pattern: "wraith::TARGET"}}
			a.runPendingTool()
			drain(t, a)
		})
	}
}

func TestReadBudgetPausesWithoutCancellingJobs(t *testing.T) {
	a := fixture(t)
	a.agentStopped = false
	a.agentReads = a.Config.Execution.MaxReads
	a.pendingTool = &contextual.ToolRequest{Tool: "poll", ID: session.ID()}
	if cmd := a.runPendingTool(); cmd != nil || !a.agentStopped || a.pendingTool != nil || a.busy {
		t.Fatal("read budget did not pause")
	}
	if !strings.Contains(a.notice, "read limit") {
		t.Fatal("budget notice missing")
	}
}

func TestPollAdvancesPastPreviouslyDeliveredLines(t *testing.T) {
	a := fixture(t)
	a.agentStopped = false
	r := runner.Record{ID: session.ID(), Session: a.Session.ID, ShareOutput: true, Finished: time.Now(), Status: "completed"}
	for i := 1; i <= 25; i++ {
		r.Output += fmt.Sprintf("line-%02d\n", i)
	}
	if err := a.history().Save(r); err != nil {
		t.Fatal(err)
	}
	calls := 0
	a.Client.HTTP.Transport = transport(func(req *http.Request) (*http.Response, error) {
		var body ollama.Request
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		text := body.Messages[len(body.Messages)-1].Content
		calls++
		if calls == 1 && (!strings.Contains(text, "line-01") || strings.Contains(text, "line-21")) {
			t.Errorf("first page: %s", text)
		}
		if calls == 2 && (!strings.Contains(text, "line-26") || strings.Contains(text, "line-01")) {
			t.Errorf("poll repeated old output: %s", text)
		}
		if calls == 3 && strings.Contains(text, "line-26") {
			t.Errorf("empty poll repeated output: %s", text)
		}
		return response(`{"done":true,"message":{"role":"assistant","content":"received"}}`), nil
	})
	for range 3 {
		a.lastPoll[r.ID] = time.Time{}
		a.pendingTool = &contextual.ToolRequest{Tool: "poll", ID: r.ID}
		a.runPendingTool()
		drain(t, a)
		r.Output += "line-26\n"
		if calls == 1 {
			if err := a.history().Save(r); err != nil {
				t.Fatal(err)
			}
		}
	}
	if a.pollNext[r.ID] != 27 {
		t.Fatalf("cursor: %d", a.pollNext[r.ID])
	}
}

func TestFailedPollSaveDoesNotAdvanceCursor(t *testing.T) {
	a := fixture(t)
	a.agentStopped = false
	r := runner.Record{ID: session.ID(), Session: a.Session.ID, ShareOutput: true, Output: "line\n", Finished: time.Now()}
	if err := a.history().Save(r); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	a.Store.Root = path
	a.pendingTool = &contextual.ToolRequest{Tool: "poll", ID: r.ID}
	if cmd := a.runPendingTool(); cmd != nil || a.pollNext[r.ID] != 0 || len(a.Session.Messages) != 0 {
		t.Fatal("failed save consumed output")
	}
}
