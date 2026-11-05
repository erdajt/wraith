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
	drain(t, a)
	if calls != 2 || a.Session.Messages[len(a.Session.Messages)-1].Content != "verified the command output" {
		t.Fatalf("continuation failed: %s", a.notice)
	}
}

func TestBlocksModeNeverOffersModelExecution(t *testing.T) {
	a := fixture(t)
	a.Config.Execution.Mode = "blocks"
	a.receive(streamMsg{Final: true, Chunk: ollama.Response{Done: true, Message: session.Message{Role: "assistant", Content: "```wraith-run\n{\"command\":\"printf x\",\"reason\":\"test\"}\n```"}}})
	if a.proposal != nil || a.busy {
		t.Fatal("blocks mode requested execution")
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
