package ui

import (
	"errors"
	"net/http"
	"testing"

	"github.com/erdajt/wraith/internal/contextual"
	"github.com/erdajt/wraith/internal/provider"
	"github.com/erdajt/wraith/internal/session"
)

func TestNativeOverflowKeepsDraftAndUsesCompactionFlow(t *testing.T) {
	for _, mode := range []string{"ask", "auto"} {
		t.Run(mode, func(t *testing.T) {
			a := fixture(t)
			a.Config.Execution.Mode, a.Config.Compact.Mode = "blocks", mode
			a.Info.Capabilities = []string{"tools"}
			a.Session.Messages = []session.Message{{Role: "user", Content: "remember the scope"}, {Role: "assistant", Content: "scope retained"}}
			a.Prompt.Set("continue")
			candidate := a.Session.Clone()
			candidate.Messages = append(candidate.Messages, session.Message{Role: "user", Content: a.Prompt.String()})
			a.Config.Context = contextual.ChatUsed(candidate, a.Config, a.Instructions) + a.Config.Predict + 128
			if err := contextual.Fits(candidate, a.runtimeConfig(), a.Instructions); err != nil {
				t.Fatal(err)
			}
			var overflow *provider.ContextError
			if _, err := a.chatBudget(candidate); !errors.As(err, &overflow) {
				t.Fatalf("fixture did not overflow native budget: %v", err)
			}
			a.Client.HTTP.Transport = transport(func(*http.Request) (*http.Response, error) { return nil, errors.New("compaction fixture stopped") })

			a.submit()
			if a.Prompt.String() != "continue" || a.pendingPrompt != "continue" || len(a.Session.Messages) != 2 {
				t.Fatal("overflow consumed draft or appended an unsendable turn")
			}
			if mode == "ask" && (a.popup == nil || a.popup.Kind != "overflow") {
				t.Fatalf("overflow picker missing: %s", a.notice)
			}
			if mode == "auto" && !a.compacting {
				t.Fatalf("automatic compaction did not start: %s", a.notice)
			}
		})
	}
}

func TestPreparationFailureDoesNotStartCompaction(t *testing.T) {
	a := fixture(t)
	a.Config.Compact.Mode = "auto"
	a.environmentCommand("set ROUTE=http://127.0.0.1:11434", false)
	a.Prompt.Set("continue")
	a.submit()
	if a.busy || a.compacting || a.pendingPrompt != "" || a.Prompt.String() != "continue" || len(a.Session.Messages) != 0 {
		t.Fatalf("privacy failure entered compaction: %s", a.notice)
	}
	if a.notice == "" {
		t.Fatal("request failure not reported")
	}
}
