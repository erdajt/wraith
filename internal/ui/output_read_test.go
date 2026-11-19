package ui

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/erdajt/wraith/internal/contextual"
	"github.com/erdajt/wraith/internal/runner"
	"github.com/erdajt/wraith/internal/session"
	"github.com/erdajt/wraith/internal/variables"
)

// outputReadFixture saves shared evidence without starting a provider request.
func outputReadFixture(t *testing.T) (*App, runner.Record) {
	t.Helper()
	a := fixture(t)
	a.agentStopped = false
	record := runner.Record{ID: session.ID(), Session: a.Session.ID, ShareOutput: true, Output: "private.example evidence\n", Finished: time.Now()}
	if err := a.history().Save(record); err != nil {
		t.Fatal(err)
	}
	a.pendingTool = &contextual.ToolRequest{Tool: "poll", ID: record.ID}
	return a, record
}

// readCompletion waits for detached output without consuming unrelated UI events.
func readCompletion(t *testing.T, a *App) outputReadMsg {
	t.Helper()
	select {
	case event := <-a.events:
		result, ok := event.(outputReadMsg)
		if !ok {
			t.Fatalf("unexpected completion: %T", event)
		}
		return result
	case <-time.After(3 * time.Second):
		t.Fatal("output reader did not finish")
		return outputReadMsg{}
	}
}

// TestOutputReadDefersForDraft keeps input usable while source capacity is occupied.
func TestOutputReadDefersForDraft(t *testing.T) {
	a, record := outputReadFixture(t)
	slots := make(chan struct{}, 2)
	slots <- struct{}{}
	slots <- struct{}{}
	a.workspaceScans = &workspaceScanStore{reads: slots}
	a.runPendingTool()
	if a.busy || a.outputRead == nil || len(a.Session.Messages) != 0 {
		t.Fatal("read blocked chat or consumed output before source access")
	}
	a.Prompt.Set("keep this draft")
	<-slots
	result := readCompletion(t, a)
	a.Update(result)
	if a.Prompt.String() != "keep this draft" || a.outputRead == nil || len(a.Session.Messages) != 0 || a.pollNext[record.ID] != 0 {
		t.Fatal("completion disturbed input or consumed an uncommitted page")
	}
	a.Client.HTTP.Transport = transport(func(*http.Request) (*http.Response, error) {
		saved, err := a.Store.Load(a.Session.ID)
		if err != nil || len(saved.Messages) != 1 || saved.Messages[0].Evidence == nil {
			t.Error("provider continued before durable evidence")
		}
		return response(`{"done":true,"message":{"role":"assistant","content":"received"}}`), nil
	})
	a.Prompt.Set("")
	a.runPendingTool()
	drain(t, a)
	if a.pollNext[record.ID] != 2 || a.agentReads != 1 {
		t.Fatal("deferred page was lost or consumed twice")
	}
}

// TestOutputReadRejectsStaleOwners discards completed and queued reads across lifecycle changes.
func TestOutputReadRejectsStaleOwners(t *testing.T) {
	for _, change := range []string{"stop", "session", "turn", "mode", "request", "close"} {
		t.Run(change, func(t *testing.T) {
			a, _ := outputReadFixture(t)
			a.workspaceScans = &workspaceScanStore{reads: make(chan struct{}, 2)}
			a.workspaceScans.reads <- struct{}{}
			a.workspaceScans.reads <- struct{}{}
			a.runPendingTool()
			switch change {
			case "stop":
				a.stopAgent()
			case "session":
				a.Session.ID = session.ID()
			case "turn":
				a.generation = session.ID()
			case "mode":
				a.Config.Execution.Mode = "blocks"
			case "request":
				a.pendingTool = nil
			case "close":
				a.Close()
			}
			a.reconcileOutputRead()
			if a.outputRead != nil {
				t.Fatal("stale reader retained its source wait")
			}
			if change != "close" {
				a.Update(readCompletion(t, a))
			}
			if len(a.Session.Messages) != 0 || a.agentReads != 0 {
				t.Fatal("stale read reached the conversation")
			}
		})
	}
}

// TestOutputReadRefreshesPrivacy rejects an old redaction snapshot and honors withdrawn sharing.
func TestOutputReadRefreshesPrivacy(t *testing.T) {
	a, record := outputReadFixture(t)
	a.runPendingTool()
	old := readCompletion(t, a)
	a.Variables = variables.NewSnapshot([]variables.Entry{{Name: "TARGET", Value: "private.example"}}, nil)
	a.Update(old)
	if a.outputRead != nil || a.pendingTool == nil || len(a.Session.Messages) != 0 {
		t.Fatal("privacy change consumed stale output")
	}
	a.runPendingTool()
	current := readCompletion(t, a)
	if current.evidence == nil || strings.Contains(current.evidence.Page.Lines[0].Text, "private.example") {
		t.Fatal("retry did not apply the new redaction snapshot")
	}
	record.OutputHidden = true
	a.pendingRuns[record.ID] = record
	a.Client.HTTP.Transport = transport(func(*http.Request) (*http.Response, error) {
		return response(`{"done":true,"message":{"role":"assistant","content":"withheld"}}`), nil
	})
	a.Update(current)
	drain(t, a)
	if a.pollNext[record.ID] != 0 || a.Session.Messages[0].Evidence != nil || !strings.Contains(a.Session.Messages[0].Content, "withheld") {
		t.Fatal("withdrawn permission exposed output")
	}
}
