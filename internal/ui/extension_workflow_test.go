package ui

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/erdajt/wraith/internal/ollama"
)

func TestModelExtensionRequestRequiresApprovalAndReturnsRedactedEvidence(t *testing.T) {
	a := pluginFixture(t, echoExtension)
	a.Config.Execution.Mode = "agent"
	a.environmentCommand("set TARGET=private.example", false)
	a.popup = nil
	a.Client = a.providerClient(a.Config)
	a.Info.Capabilities = []string{"tools"}
	calls := 0
	var continuation string
	a.Client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		calls++
		var request ollama.Request
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if len(request.Tools) != 3 || request.Tools[2].Function.Name != "inspect" {
			t.Errorf("extension missing from catalog: %+v", request.Tools)
		}
		if calls > 1 {
			for _, message := range request.Messages {
				if strings.Contains(message.Content, "command result") {
					continuation = message.Content
				}
			}
			return response(`{"done":true,"done_reason":"stop","message":{"content":"finished"}}`), nil
		}
		return response(`{"message":{"tool_calls":[{"function":{"name":"inspect","arguments":{"message":"wraith::TARGET"}}}]}}` + "\n" + `{"done":true,"done_reason":"stop"}`), nil
	})
	a.Prompt.Set("inspect the configured target")
	a.submit()
	drain(t, a)
	if a.proposal == nil || a.proposal.Extension == nil || len(a.jobs.Snapshot()) != 0 || !strings.Contains(a.proposal.Resolved, "private.example") {
		t.Fatalf("model request did not reach concrete approval: %s", a.notice)
	}
	id := a.proposal.launch.ID
	a.Prompt.Set("draft")
	a.approvalAction("run")
	err := runTransport(t, a, func() error {
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for {
			for _, job := range a.jobs.Snapshot() {
				if !job.Record.Finished.IsZero() {
					return job.Err
				}
			}
			select {
			case <-ticker.C:
			case <-t.Context().Done():
				return t.Context().Err()
			}
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	finishJobs(t, a)
	candidate, _ := a.pendingEvidence()
	a.Session = candidate
	a.Prompt.Set("")
	a.startChat()
	drain(t, a)
	if continuation == "" || !strings.Contains(continuation, id) || !strings.Contains(continuation, "wraith::TARGET") || strings.Contains(continuation, "private.example") || !strings.Contains(continuation, "progress") {
		t.Fatalf("tool evidence lost identity or privacy: %q", continuation)
	}
}
