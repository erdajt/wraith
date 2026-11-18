package ui

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type callbackProgram struct {
	app   *App
	start tea.Cmd
}

// Init starts a callback chain in the real terminal event loop.
func (m callbackProgram) Init() tea.Cmd { return m.start }

// View suppresses rendering while callback delivery remains asynchronous.
func (m callbackProgram) View() string { return "" }

// Update stops once the chain completes or reports a guarded failure.
func (m callbackProgram) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, cmd := m.app.Update(msg)
	if m.app.notice == "complete" || strings.HasPrefix(m.app.notice, "lua") || strings.Contains(m.app.notice, "chain limit") {
		return m, tea.Quit
	}
	return m, cmd
}

func TestNestedCallbacksCompleteBeforeCallerEffects(t *testing.T) {
	for _, test := range []struct {
		name, source, want, notice string
	}{
		{"draft", `wraith.commands.create("inner", function() wraith.prompt.set("inner") end)`, "after", "complete"},
		{"empty", `wraith.commands.create("inner", function() end)`, "after", "complete"},
		{"nested", `
			wraith.commands.create("leaf", function() wraith.prompt.set("leaf") end)
			wraith.commands.create("inner", function() wraith.action("command.leaf"); wraith.prompt.set("inner") end)
		`, "after", "complete"},
		{"error", `wraith.commands.create("inner", function() wraith.prompt.set("discarded"); error("child failed") end)`, "before", "child failed"},
		{"timeout", `wraith.commands.create("inner", function() while true do end end)`, "before", "deadline"},
		{"depth", `wraith.commands.create("inner", function() wraith.action("command.inner") end)`, "before", "chain limit"},
	} {
		t.Run(test.name, func(t *testing.T) {
			a := pluginFixture(t, test.source+`
				wraith.commands.create("outer", function()
					wraith.action("command.inner")
					wraith.prompt.set("after")
					wraith.notify("complete")
				end)
			`)
			a.Prompt.Set("before")
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			program := tea.NewProgram(callbackProgram{a, a.runCommand("outer")}, tea.WithContext(ctx), tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer(), tea.WithoutSignalHandler())
			if _, err := program.Run(); err != nil {
				t.Fatal(err)
			}
			if a.Prompt.String() != test.want || !strings.Contains(a.notice, test.notice) {
				t.Fatalf("draft=%q notice=%q", a.Prompt.String(), a.notice)
			}
		})
	}
}

func TestStaleNestedCallbackCannotResumeCaller(t *testing.T) {
	for _, change := range []string{"draft", "focus", "reload"} {
		t.Run(change, func(t *testing.T) {
			a := pluginFixture(t, `
				wraith.commands.create("inner", function() wraith.prompt.set("inner") end)
				wraith.commands.create("outer", function() wraith.action("command.inner"); wraith.prompt.set("after") end)
			`)
			_, child := a.Update(a.runCommand("outer")())
			if child == nil {
				t.Fatal("nested callback was not submitted")
			}
			switch change {
			case "draft":
				a.Prompt.Set("user input")
			case "focus":
				a.focus = "chat"
			case "reload":
				a.plugins = nil
			}
			before := a.Prompt.String()
			_, next := a.Update(child())
			if next != nil || a.Prompt.String() != before {
				t.Fatal("stale child resumed its caller")
			}
		})
	}
}

func TestSupersededEmptyEventLeavesStatusUntouched(t *testing.T) {
	a := pluginFixture(t, `wraith.events.on("ready", function() end)`)
	result := a.callPlugin("", "ready", "")()
	a.Prompt.Set("new draft")
	a.notice = "current status"
	a.Update(result)
	if a.notice != "current status" {
		t.Fatal("empty event replaced the current status")
	}
}
