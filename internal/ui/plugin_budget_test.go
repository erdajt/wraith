package ui

import (
	"context"
	"net/http"
	"testing"

	"github.com/erdajt/wraith/internal/provider"
)

func TestTransformedBudgetRestoresDraftWithoutCallingProvider(t *testing.T) {
	for _, typing := range []bool{false, true} {
		t.Run(map[bool]string{false: "restore", true: "preserve-new-input"}[typing], func(t *testing.T) {
			a := fixture(t)
			a.Client.Transform = func(_ context.Context, _ string, data provider.PromptData) (provider.PromptData, error) {
				data.Options.Context = 128
				return data, nil
			}
			a.Client.HTTP.Transport = transport(func(*http.Request) (*http.Response, error) {
				t.Error("over-budget transform reached provider")
				return response(""), nil
			})
			a.Prompt.Set("inspect this workspace")
			before := len(a.Session.Messages)
			cmd := a.submit()
			if cmd == nil {
				t.Fatal("submission did not reach transformed budget check")
			}
			if typing {
				a.Prompt.Set("new draft")
			}
			a.Update(cmd())
			if a.busy || a.popup == nil || a.popup.Kind != "overflow" {
				t.Fatalf("overflow did not enter recovery: %s", a.notice)
			}
			if typing {
				if a.Prompt.String() != "new draft" || len(a.Session.Messages) != before+1 {
					t.Fatal("recovery discarded new input or the submitted prompt")
				}
			} else if a.Prompt.String() != "inspect this workspace" || len(a.Session.Messages) != before || a.pendingPrompt == "" {
				t.Fatal("failed submission was not restored as an unsent draft")
			}
		})
	}
}
