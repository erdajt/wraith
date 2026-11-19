package ui

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type scheduledProgram struct {
	app     *App
	start   tea.Cmd
	failure bool
}

func (p *scheduledProgram) Init() tea.Cmd { return p.start }
func (p *scheduledProgram) View() string  { return "" }

func (p *scheduledProgram) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	_, cmd := p.app.Update(message)
	p.failure = p.failure || strings.Contains(p.app.notice, "expected failure")
	if p.app.notice == "complete" {
		return p, tea.Quit
	}
	return p, cmd
}

// runScheduledProgram exercises deferred delivery through the actual UI message loop.
func runScheduledProgram(t *testing.T, a *App, start tea.Cmd) *scheduledProgram {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	model := &scheduledProgram{app: a, start: start}
	program := tea.NewProgram(model, tea.WithContext(ctx), tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer(), tea.WithoutSignalHandler())
	if _, err := program.Run(); err != nil {
		t.Fatalf("scheduled callbacks did not finish: %v; %s", err, a.notice)
	}
	return model
}

func TestLuaScheduledCallbacksSeeAppliedSiblingChanges(t *testing.T) {
	a := pluginFixture(t, `wraith.commands.create("start", function()
  wraith.schedule(function(ctx)
    assert(ctx.prompt == "parent")
    wraith.prompt.set("first")
    wraith.theme.use("nord")
    wraith.commands.create("late", function() end)
  end)
  wraith.schedule(function(ctx)
    assert(ctx.prompt == "first")
    assert(wraith.config.get("theme") == "nord")
    wraith.prompt.set("second")
  end)
  wraith.schedule(function(ctx)
    assert(ctx.prompt == "second")
    wraith.notify("complete")
  end)
  wraith.prompt.set("parent")
end)`)
	runScheduledProgram(t, a, a.runCommand("start"))
	if a.Prompt.String() != "second" || a.Config.Theme != "nord" || len(a.pluginCommands("late")) != 1 {
		t.Fatal("scheduled sibling changes were lost")
	}
}

func TestLuaScheduledOrderSurvivesNestedEffectChains(t *testing.T) {
	a := pluginFixture(t, `local order = ""
wraith.commands.create("inner", function()
  wraith.schedule(function() order = order .. "b" end)
end)
wraith.commands.create("start", function()
  wraith.schedule(function() order = order .. "a" end)
  wraith.action("command.inner")
  wraith.schedule(function() assert(order == "ab"); wraith.notify("complete") end)
end)`)
	runScheduledProgram(t, a, a.runCommand("start"))
}

func TestLuaScheduledFailureDoesNotCancelQueuedSibling(t *testing.T) {
	a := pluginFixture(t, `wraith.commands.create("start", function()
  wraith.schedule(function()
    wraith.prompt.set("discard")
    error("expected failure")
  end)
  wraith.schedule(function(ctx)
    assert(ctx.prompt == "")
    wraith.notify("complete")
  end)
end)`)
	program := runScheduledProgram(t, a, a.runCommand("start"))
	if !program.failure || a.Prompt.String() != "" {
		t.Fatal("scheduled failure was hidden or kept its failed effects")
	}
}

func TestLuaAbandonedEffectsReleaseScheduledCallbacks(t *testing.T) {
	a := pluginFixture(t, `wraith.commands.create("start", function()
  wraith.prompt.set("blocked")
  wraith.schedule(function() error("must not run") end)
end)`)
	a.popup = &menu{Kind: "approval"}
	for range 200 {
		a.Update(a.callPlugin(a.pluginManifest.Commands[0].Callback, "", "")())
		if strings.Contains(a.notice, "limit") {
			t.Fatalf("abandoned schedules retained pending slots: %s", a.notice)
		}
	}
	if a.Prompt.String() != "" {
		t.Fatal("scheduled batch bypassed approval ownership")
	}
}

func TestLuaScheduledSiblingContinuesAfterEditorFinishes(t *testing.T) {
	for _, ending := range []string{"saved", "cancelled", "failed", "stale"} {
		t.Run(ending, func(t *testing.T) {
			a := pluginFixture(t, `wraith.commands.create("start", function()
  wraith.schedule(function() wraith.action("editor.open") end)
  wraith.schedule(function() wraith.notify("complete") end)
end)`)
			a.Prompt.Set("draft")
			_, queued := a.Update(a.runCommand("start")())
			_, callback := a.Update(queued())
			_, editor := a.Update(callback())
			if editor == nil || !a.editing || a.editorFile == nil || len(a.editorDeferred) != 1 {
				t.Fatal("scheduled callback did not hold its sibling for the editor")
			}
			result := editedMsg{Kind: "prompt", File: a.editorFile}
			switch ending {
			case "saved":
				if err := os.WriteFile(a.editorFile.Path, []byte("saved"), 0600); err != nil {
					t.Fatal(err)
				}
			case "failed":
				result.Err = errors.New("editor failed")
			case "stale":
				a.Prompt.Set("new draft")
			}
			_, resume := a.Update(result)
			runScheduledProgram(t, a, resume)
			if a.editing || len(a.editorDeferred) != 0 {
				t.Fatal("editor continuation retained deferred work")
			}
		})
	}
}
