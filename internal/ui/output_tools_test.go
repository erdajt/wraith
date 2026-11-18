package ui

import (
	"context"
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

func TestPollSkipsOversizedLineWithoutStalling(t *testing.T) {
	a := fixture(t)
	a.Config.Execution.OutputContext = 512
	a.agentStopped = false
	r := runner.Record{ID: session.ID(), Session: a.Session.ID, ShareOutput: true, Output: strings.Repeat("x", 1024) + "\nnext\n", Finished: time.Now()}
	if err := a.history().Save(r); err != nil {
		t.Fatal(err)
	}

	a.Client.HTTP.Transport = transport(func(*http.Request) (*http.Response, error) {
		return response(`{"done":true,"message":{"role":"assistant","content":"observed"}}`), nil
	})
	a.pendingTool = &contextual.ToolRequest{Tool: "poll", ID: r.ID}
	a.runPendingTool()
	drain(t, a)
	page := a.Session.Messages[0].Evidence.Page
	if a.pollNext[r.ID] != 3 || len(page.Lines) != 2 || !page.Truncated || !strings.Contains(page.Lines[0].Text, "exceeds") || page.Lines[1].Text != "next" {
		t.Fatalf("poll failed to advance: %+v, cursor %d", page, a.pollNext[r.ID])
	}
}

func TestOutputToolsWaitForDurableFinalResult(t *testing.T) {
	a := fixture(t)
	a.agentStopped = false
	r := runner.Record{ID: session.ID(), Session: a.Session.ID, Command: "sleep 30", Shell: "/bin/sh", ShareOutput: true}

	if err := a.jobs.Start(context.Background(), r, time.Minute); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(a.history().Root, r.ID+".json.gz")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}

	a.jobs.Close()

	if jobs := a.jobs.Snapshot(); len(jobs) != 1 || jobs[0].Err == nil {
		t.Fatalf("missing persistence failure: %+v", jobs)
	}

	a.pendingTool = &contextual.ToolRequest{Tool: "poll", ID: r.ID}

	if cmd := a.runPendingTool(); cmd != nil || !a.agentStopped || a.pendingTool != nil || len(a.Session.Messages) != 0 {
		t.Fatal("unsaved final output reached conversation")
	}

	if _, err := a.jobRecord(r.ID); err != nil {
		t.Fatalf("local inspection lost: %v", err)
	}
}

func TestOutputReadDefersBeforeConsumingFullContext(t *testing.T) {
	a := fixture(t)
	a.agentStopped = false
	a.Session.Messages = []session.Message{{Role: "user", Content: strings.Repeat("long context ", 3000)}}
	id := session.ID()
	a.pendingTool = &contextual.ToolRequest{Tool: "poll", ID: id}
	a.runPendingTool()

	if a.popup == nil || a.popup.Kind != "overflow" || a.pendingTool == nil || a.agentReads != 0 || a.pollNext[id] != 0 || len(a.Session.Messages) != 1 {
		t.Fatal("read consumed context before compaction")
	}

	a.menuKey(key("n"))

	if a.pendingTool != nil || a.agentReads != 0 || len(a.Session.Messages) != 1 {
		t.Fatal("declined compaction consumed output")
	}
}

func TestFailedToolCompactionDoesNotRetryEveryTick(t *testing.T) {
	a := fixture(t)
	a.agentStopped = false
	a.pendingTool = &contextual.ToolRequest{Tool: "poll", ID: session.ID()}
	a.finishCompact(compactMsg{Err: context.Canceled})

	if a.pendingTool != nil || !a.agentStopped || a.agentReads != 0 {
		t.Fatal("failed compaction retained automatic read")
	}
}

func TestLargeContextDoesNotOverflowOutputBudget(t *testing.T) {
	a := fixture(t)
	a.Config = a.Config.WithModelContext(a.Session.Model, int(^uint(0)>>1)/2)
	a.agentStopped = false
	r := runner.Record{ID: session.ID(), Session: a.Session.ID, ShareOutput: true, Output: "evidence\n", Finished: time.Now()}
	if err := a.history().Save(r); err != nil {
		t.Fatal(err)
	}

	a.Client.HTTP.Transport = transport(func(*http.Request) (*http.Response, error) {
		return response(`{"done":true,"message":{"role":"assistant","content":"observed"}}`), nil
	})
	a.pendingTool = &contextual.ToolRequest{Tool: "poll", ID: r.ID}
	a.runPendingTool()
	drain(t, a)

	if a.pendingTool != nil || len(a.Session.Messages) == 0 || a.Session.Messages[0].Evidence == nil {
		t.Fatal("valid large context incorrectly deferred output read")
	}
	if page := a.Session.Messages[0].Evidence.Page; len(page.Lines) != 1 || page.Lines[0].Text != "evidence" {
		t.Fatalf("lost output evidence: %+v", page)
	}
}
