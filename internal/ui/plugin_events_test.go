package ui

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/session"

	tea "github.com/charmbracelet/bubbletea"
)

func TestLuaWindowEventsSeeUpdatedGeometry(t *testing.T) {
	a := pluginFixture(t, `wraith.events.on("window_changed", function() local views = wraith.windows.list(); assert(views[1].visible and views[1].width > 0); wraith.notify(tostring(#views)) end)`)
	a.width, a.height, a.focus = 120, 40, "chat"
	a.key(tea.KeyMsg{Type: tea.KeyCtrlW})
	_, cmd := a.Update(key("v"))
	if cmd == nil {
		t.Fatal("window event was not queued")
	}
	a.Update(cmd())
	if a.notice != "2" {
		t.Fatalf("event saw outdated window geometry: %s", a.notice)
	}
}

func TestLuaCallbackChainsStopWithoutBlockingInput(t *testing.T) {
	a := pluginFixture(t, `wraith.commands.create("repeat", function() wraith.action("command.repeat") end)`)
	cmd := a.runCommand("repeat")
	for i := 0; cmd != nil && i < 20; i++ {
		_, cmd = a.Update(cmd())
	}
	if cmd != nil || a.notice != "plugin callback chain limit reached" {
		t.Fatalf("recursive callback chain did not stop: %s", a.notice)
	}
	a.key(key("draft"))
	if a.Prompt.String() != "draft" {
		t.Fatal("callback chain blocked normal input")
	}
}

// applyPluginEvents consumes event effects without dispatching unrelated background work.
func applyPluginEvents(t *testing.T, a *App, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		return
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, child := range msg {
			applyPluginEvents(t, a, child)
		}
	case pluginResultMsg:
		a.Update(msg)
	}
}

func TestLuaShellCompletionDoesNotEmitResponseEvent(t *testing.T) {
	a := pluginFixture(t, `wraith.events.on("response_finished", function() wraith.notify("model response event") end)`)
	a.Config.Execution.Mode, a.Config.Execution.AutoShare = "blocks", false
	a.proposal = &proposal{Command: "printf local", Language: "sh", Source: "block"}
	a.runApproved()
	if !a.executing || !a.busy {
		t.Fatal("foreground job did not start")
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		jobs := a.jobs.Snapshot()
		if len(jobs) == 1 && !jobs[0].Record.Finished.IsZero() {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("job did not finish")
		}
		time.Sleep(time.Millisecond)
	}
	_, cmd := a.Update(tick(time.Now()))
	applyPluginEvents(t, a, cmd)
	if a.notice == "model response event" {
		t.Fatal("foreground shell completion emitted response_finished")
	}
}

func TestLuaResponseEventsFollowAcceptedFinalChunks(t *testing.T) {
	for _, test := range []struct {
		name    string
		message streamMsg
		want    bool
	}{
		{"completed", streamMsg{Serial: 2, Final: true}, true},
		{"cancelled", streamMsg{Serial: 2, Final: true, Err: context.Canceled}, true},
		{"stale", streamMsg{Serial: 1, Final: true}, false},
		{"partial", streamMsg{Serial: 2}, false},
		{"repair", streamMsg{Serial: 2, Retry: 1}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			a := pluginFixture(t, `wraith.events.on("response_finished", function() wraith.notify("model response event") end)`)
			a.streamSerial, a.busy = 2, true
			a.events <- nil
			_, cmd := a.Update(test.message)
			applyPluginEvents(t, a, cmd)
			if got := a.notice == "model response event"; got != test.want {
				t.Fatalf("response event = %v, want %v", got, test.want)
			}
		})
	}
}

func TestLuaResponseEventSurvivesImmediateToolContinuation(t *testing.T) {
	a := pluginFixture(t, `wraith.events.on("response_finished", function() wraith.notify("model response event") end)`)
	a.Config.Execution.Mode, a.Config.Thinking = "agent", "off"
	a.Client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		return response(`{"message":{"content":"continued"},"done":true}`), nil
	})
	a.streamSerial, a.busy, a.agentStopped = 2, true, false
	request := "```wraith-tool\n{\"tool\":\"poll\",\"id\":\"0123456789abcdef01234567\"}\n```"
	_, cmd := a.Update(streamMsg{Serial: 2, Final: true, Chunk: ollama.Response{Message: session.Message{Role: "assistant", Content: request}}})
	if !a.busy || a.streamSerial != 3 {
		t.Fatalf("tool did not start another response: %s", a.notice)
	}
	applyPluginEvents(t, a, cmd)
	if a.notice != "model response event" {
		t.Fatalf("response event omitted during continuation: %s", a.notice)
	}
}
