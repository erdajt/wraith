package ui

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/contextual"
	"github.com/erdajt/wraith/internal/editor"
	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/session"
)

func TestNativeRequestRequiresDecisionAndPreservesEvidencePolicy(t *testing.T) {
	for _, decision := range []string{"run", "run-hidden", "edit", "cancel"} {
		t.Run(decision, func(t *testing.T) {
			a := fixture(t)
			a.Info.Capabilities = []string{"tools"}
			marker := filepath.Join(t.TempDir(), "executed")
			command := "printf original; touch " + editor.Quote(marker)
			calls := 0
			var continuation string
			a.Client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
				calls++
				var request ollama.Request
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
				}
				if len(request.Tools) != 2 {
					t.Error("UI omitted model tool capability")
				}
				if calls > 1 {
					for _, message := range request.Messages {
						if strings.Contains(message.Content, "command result") {
							continuation = message.Content
						}
					}
					return response(`{"done":true,"done_reason":"stop","message":{"content":"finished"}}`), nil
				}
				args, _ := json.Marshal(contextual.CommandRequest{Command: command, Reason: "verify approval"})
				return response(`{"message":{"tool_calls":[{"function":{"name":"request_command","arguments":` + string(args) + `}}]}}` + "\n" + `{"done":true,"done_reason":"stop"}`), nil
			})
			a.Prompt.Set("run the local fixture")
			a.submit()
			drain(t, a)
			if a.proposal == nil || a.popup == nil || a.popup.Kind != "approval" || a.proposal.Reason != "verify approval" || len(a.jobs.Snapshot()) != 0 {
				t.Fatalf("missing approval: %s", a.notice)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatal("native call executed without approval")
			}

			a.Prompt.Set("keep chat available")
			if decision == "edit" {
				file, err := editor.Prepare(a.Paths.Cache, ".sh", command)
				if err != nil {
					t.Fatal(err)
				}
				command = "printf edited; touch " + editor.Quote(marker)
				if err := os.WriteFile(file.Path, []byte(command), 0600); err != nil {
					t.Fatal(err)
				}
				a.finishEdit(editedMsg{Kind: "run", Language: "sh", File: file})
			} else {
				a.approvalAction(decision)
			}
			if decision == "cancel" {
				if !a.agentStopped || a.proposal != nil || len(a.jobs.Snapshot()) != 0 || calls != 1 {
					t.Fatal("denial did not stop the turn")
				}
				if _, err := os.Stat(marker); !os.IsNotExist(err) {
					t.Fatal("denied command executed")
				}
				return
			}
			if a.busy || a.Prompt.String() != "keep chat available" {
				t.Fatal("command blocked the conversation")
			}
			finishJobs(t, a)
			records, err := a.history().List()
			if err != nil || len(records) != 1 {
				t.Fatalf("command must run exactly once: %v", err)
			}
			record, err := a.history().Load(records[0].ID)
			if err != nil || record.Command != command || record.Status != "completed" || !record.Background || record.OutputHidden != (decision == "run-hidden") {
				t.Fatalf("approval policy lost: %+v, %v", record, err)
			}
			candidate, _ := a.pendingEvidence()
			a.Session = candidate
			a.Prompt.Set("")
			a.startChat()
			drain(t, a)
			if continuation == "" || !strings.Contains(continuation, record.ID) || !strings.Contains(continuation, command) {
				t.Fatal("command input and status did not reach the model")
			}
			if decision == "run-hidden" {
				if strings.Contains(continuation, "output:\noriginal") {
					t.Fatal("hidden output reached the model")
				}
			} else if !strings.Contains(continuation, "output:\n"+record.Output) {
				t.Fatal("approved output did not reach the model")
			}
		})
	}
}

func TestNativeRequestCannotReappearOnSessionReload(t *testing.T) {
	a := fixture(t)
	content, err := contextual.NormalizeAction("agent", "", []ollama.Call{{Name: "request_command", Arguments: json.RawMessage(`{"command":"printf ready","reason":"check"}`)}})
	if err != nil {
		t.Fatal(err)
	}
	a.Session.Messages = []session.Message{{Role: "assistant", Content: content}}
	if err := a.Store.Save(a.Session); err != nil {
		t.Fatal(err)
	}
	saved, err := a.Store.Load(a.Session.ID)
	if err != nil {
		t.Fatal(err)
	}
	resumed, err := New(a.Config, a.Paths, saved, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(resumed.Close)
	if resumed.proposal != nil || len(resumed.jobs.Snapshot()) > 0 {
		t.Fatal("resuming a historical request reactivated execution")
	}
}
