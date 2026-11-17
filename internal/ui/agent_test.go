package ui

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/session"
)

func TestAgentApprovesRunsAndContinuesWithEvidence(t *testing.T) {
	a := fixture(t)
	a.Config.Execution.AutoShare = true
	calls := 0
	a.Client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		var req ollama.Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		calls++
		content := "```wraith-run\n{\"command\":\"printf evidence\",\"reason\":\"inspect local output\"}\n```"
		if calls == 2 {
			content = "waiting for the background result"
		}
		if calls == 3 {
			if !strings.Contains(req.Messages[len(req.Messages)-1].Content, "output:\nevidence") {
				t.Error("command output not supplied")
			}
			content = "verified the command output"
		}
		b, _ := json.Marshal(ollama.Response{Done: true, Message: session.Message{Role: "assistant", Content: content}})
		return response(string(b)), nil
	})
	a.Prompt.Set("inspect local output")
	a.submit()
	drain(t, a)
	if calls != 1 || a.popup == nil || a.popup.Kind != "approval" || a.busy {
		t.Fatal("model request did not stop for approval")
	}
	a.approvalAction("run")
	finishJobs(t, a)
	if calls != 3 || a.Session.Messages[len(a.Session.Messages)-1].Content != "verified the command output" {
		t.Fatalf("continuation failed: %s", a.notice)
	}
}

func TestBlocksModeOffersApprovalWithoutAutomaticOutputSharing(t *testing.T) {
	a := fixture(t)
	a.Config.Execution.Mode = "blocks"
	a.receive(streamMsg{Final: true, Chunk: ollama.Response{Done: true, Message: session.Message{Role: "assistant", Content: "```wraith-run\n{\"command\":\"printf x\",\"reason\":\"test\"}\n```"}}})
	if a.proposal == nil || a.popup == nil || a.popup.Kind != "approval" || a.busy {
		t.Fatal("blocks mode did not offer approval")
	}
}

func TestAgentStopsAtStepBudget(t *testing.T) {
	a := fixture(t)
	a.agentSteps = a.Config.Execution.MaxSteps
	a.receive(streamMsg{Final: true, Chunk: ollama.Response{Done: true, Message: session.Message{Role: "assistant", Content: "```wraith-run\n{\"command\":\"printf x\",\"reason\":\"test\"}\n```"}}})
	if a.proposal != nil || !strings.Contains(a.notice, "step limit") {
		t.Fatal("step budget bypassed")
	}
}

func TestBackgroundApprovalReturnsReceiptAndDenialStopsItsTurn(t *testing.T) {
	a := fixture(t)
	a.generation = session.ID()
	a.Client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		var req ollama.Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		last := req.Messages[len(req.Messages)-1].Content
		if !strings.Contains(last, "background command accepted") || !strings.Contains(last, "id:") {
			t.Errorf("missing start receipt: %s", last)
		}
		return response(`{"done":true,"message":{"role":"assistant","content":"waiting for the result"}}`), nil
	})
	a.approve(proposal{Command: "sleep 30", Continue: true, Background: true})
	if !strings.Contains(a.popup.Title, "background") {
		t.Fatal("background launch not disclosed")
	}
	a.runApproved()
	if a.executing {
		t.Fatal("background job blocked chat")
	}
	drain(t, a)
	a.approve(proposal{Command: "printf declined", Continue: true})
	a.declineCommand()
	a.jobs.Close()
	a.syncJobs()
	if !a.agentStopped || a.busy {
		t.Fatal("denial resumed the agent")
	}
	records, err := a.history().List()
	if err != nil || len(records) != 1 || records[0].Status != "cancelled" {
		t.Fatalf("turn jobs survived denial: %v %v", records, err)
	}
}

func TestCancelledStreamCannotOfferExecutionOrResumeTools(t *testing.T) {
	for _, content := range []string{
		"```wraith-run\n{\"command\":\"printf late\",\"reason\":\"queued response\"}\n```",
		"```wraith-tool\n{\"tool\":\"poll\",\"id\":\"0123456789abcdef01234567\"}\n```",
	} {
		t.Run(content, func(t *testing.T) {
			a := fixture(t)
			a.busy = true
			a.stopAgent()
			a.receive(streamMsg{Final: true, Chunk: ollama.Response{Done: true, Message: session.Message{Role: "assistant", Content: content}}})
			if a.proposal != nil || a.pendingTool != nil || a.busy || a.agentReads != 0 {
				t.Fatal("cancelled turn continued")
			}
			if n := len(a.Session.Messages); n > 0 && !a.Session.Messages[n-1].Interrupted {
				t.Fatal("cancelled response not marked interrupted")
			}
		})
	}
}

func TestOrdinaryApprovedRunFollowsExecutionMode(t *testing.T) {
	for _, mode := range []string{"agent", "blocks"} {
		t.Run(mode, func(t *testing.T) {
			a := fixture(t)
			a.Config.Execution.Mode = mode
			calls := 0
			a.Client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
				calls++
				return response(`{"done":true,"message":{"role":"assistant","content":"observed output"}}`), nil
			})
			a.approve(proposal{Command: "printf result", Source: "block"})
			a.runApproved()
			finishJobs(t, a)
			if mode == "agent" && (calls != 2 || len(a.Session.SharedRuns) != 1) {
				t.Fatal("agent mode did not observe ordinary run")
			}
			if mode == "blocks" && (calls != 0 || len(a.Session.Messages) != 0) {
				t.Fatal("blocks output shared automatically")
			}
		})
	}
}
