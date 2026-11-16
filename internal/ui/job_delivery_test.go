package ui

import (
	"encoding/json"
	"github.com/erdajt/wraith/internal/ollama"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/erdajt/wraith/internal/runner"
	"github.com/erdajt/wraith/internal/session"
)

func TestCompletedEvidenceWaitsForDraftAndDoesNotCrossSessions(t *testing.T) {
	a := fixture(t)
	r := runner.Record{ID: session.ID(), Session: a.Session.ID, Agent: true, ShareOutput: true, Output: "evidence", Finished: time.Now()}
	a.pendingRuns[r.ID] = r
	a.Prompt.Set("unsent draft")
	a.deliverRuns()
	if len(a.Session.Messages) != 0 || a.Prompt.String() != "unsent draft" {
		t.Fatal("completion interrupted draft")
	}
	a.Prompt.Set("")
	original := a.Session
	a.Session = session.New("other")
	a.deliverRuns()
	if len(a.Session.Messages) != 0 {
		t.Fatal("output crossed session boundary")
	}
	a.Session = original
	a.agentStopped = true
	a.deliverRuns()
	a.pendingRuns[r.ID] = r
	a.deliverRuns()
	if len(a.Session.Messages) != 1 || len(a.Session.DeliveredRuns) != 1 || a.busy {
		t.Fatal("duplicate evidence or stopped turn resumed")
	}
	saved, err := a.Store.Load(original.ID)
	if err != nil || len(saved.DeliveredRuns) != 1 {
		t.Fatal("delivery not durable")
	}
}

func TestCompletionDoesNotInterruptEditorOrSelection(t *testing.T) {
	a := fixture(t)
	r := runner.Record{ID: session.ID(), Session: a.Session.ID, Agent: true, Finished: time.Now()}
	a.pendingRuns[r.ID] = r
	a.editing = true
	a.deliverRuns()
	a.editing = false
	a.showRun(r, "text")
	a.popup.Reader.Frozen = true
	a.deliverRuns()
	if len(a.Session.Messages) != 0 || len(a.pendingRuns) != 1 {
		t.Fatal("interactive barrier ignored")
	}
}

func TestHiddenResultDisclosesStatusWithoutOutputOrError(t *testing.T) {
	a := fixture(t)
	a.agentStopped = true
	r := runner.Record{ID: session.ID(), Session: a.Session.ID, Agent: true, ShareOutput: true, OutputHidden: true, Command: "printf result", Status: "failed", Error: "private-error", Output: "private-output", Finished: time.Now()}
	a.pendingRuns[r.ID] = r
	a.deliverRuns()
	got := a.Session.Messages[0].Content
	if strings.Contains(got, "private-") || !strings.Contains(got, "output withheld") || len(a.Session.SharedRuns) != 0 {
		t.Fatalf("hidden result: %s", got)
	}
}

func TestPendingResultIsIncludedWithNextSubmittedDraft(t *testing.T) {
	a := fixture(t)
	r := runner.Record{ID: session.ID(), Session: a.Session.ID, Agent: true, ShareOutput: true, Output: "queued evidence", Finished: time.Now()}
	a.pendingRuns[r.ID] = r
	a.Client.HTTP.Transport = transport(func(req *http.Request) (*http.Response, error) {
		var body ollama.Request
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		n := len(body.Messages)
		if n < 3 || !strings.Contains(body.Messages[n-2].Content, "queued evidence") || body.Messages[n-1].Content != "use the result" {
			t.Errorf("pending result not delivered with draft: %+v", body.Messages)
		}
		return response(`{"done":true,"message":{"role":"assistant","content":"done"}}`), nil
	})
	a.Prompt.Set("use the result")
	a.submit()
	drain(t, a)
	if len(a.Session.DeliveredRuns) != 1 {
		t.Fatal("delivery id missing")
	}
}

func TestReconciliationPersistsEvidenceWithoutRestartingAgent(t *testing.T) {
	a := fixture(t)
	r := runner.Record{ID: session.ID(), Session: a.Session.ID, Agent: true, ShareOutput: true, Output: "completed offline", Finished: time.Now()}
	if err := a.history().Save(r); err != nil {
		t.Fatal(err)
	}
	a.reconcileJobs()
	if cmd := a.deliverRuns(); cmd != nil || a.busy {
		t.Fatal("reconciliation restarted agent")
	}
	a.reconcileJobs()
	a.deliverRuns()
	if len(a.Session.Messages) != 1 || !strings.Contains(a.Session.Messages[0].Content, "completed offline") {
		t.Fatal("reconciliation lost or duplicated output")
	}
}

func TestCommandEvidenceKeepsTrustedIdAcrossProviderDispatch(t *testing.T) {
	a := fixture(t)
	a.environmentCommand("set TOKEN=ab", false)
	a.environmentCommand("set FIELD=line", false)
	a.popup = nil
	r := runner.Record{ID: strings.Repeat("ab", 12), Session: a.Session.ID, ShareOutput: true, Agent: true, Output: "ab line\n", Finished: time.Now()}
	a.pendingRuns[r.ID] = r
	a.agentStopped = false
	a.Client.HTTP.Transport = transport(func(req *http.Request) (*http.Response, error) {
		var body ollama.Request
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		text := body.Messages[len(body.Messages)-1].Content
		if !strings.Contains(text, r.ID) || strings.Contains(text, "ab line") || !strings.Contains(text, "wraith::TOKEN wraith::FIELD") {
			t.Errorf("evidence corrupted: %s", text)
		}
		return response(`{"done":true,"message":{"role":"assistant","content":"done"}}`), nil
	})
	a.deliverRuns()
	drain(t, a)
	if a.Session.Messages[0].Evidence == nil {
		t.Fatal("evidence metadata missing")
	}
}
