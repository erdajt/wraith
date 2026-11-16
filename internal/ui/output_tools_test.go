package ui

import (
	"encoding/json"
	"net/http"
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
	a.agentReads = a.Config.Execution.MaxReads
	a.pendingTool = &contextual.ToolRequest{Tool: "poll", ID: session.ID()}
	if cmd := a.runPendingTool(); cmd != nil || !a.agentStopped || a.pendingTool != nil || a.busy {
		t.Fatal("read budget did not pause")
	}
	if !strings.Contains(a.notice, "read limit") {
		t.Fatal("budget notice missing")
	}
}
