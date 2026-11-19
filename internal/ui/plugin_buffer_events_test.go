package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/erdajt/wraith/internal/plugin"
)

// TestBufferObserversReceiveTextChangesWithoutDecorationFeedback exercises the complete event bridge.
func TestBufferObserversReceiveTextChangesWithoutDecorationFeedback(t *testing.T) {
	a := pluginFixture(t, `local calls=0
wraith.events.on("buffer_changed",function(ctx)
 calls=calls+1
 assert(calls==1)
 local id=wraith.buffers.native("prompt")
 local b=wraith.buffers.get(id)
 assert(b.lines[1]=="draft")
 wraith.buffers.set_spans(id,{{start={line=0,column=0},["end"]={line=0,column=5},style={bold=true}}})
 wraith.notify("decorated")
end)`)
	a.syncNativeSurfaces()
	a.Prompt.Set("draft")
	_, cmd := a.Update(struct{}{})
	applyPluginEvents(t, a, cmd)
	if a.notice != "decorated" {
		t.Fatal(a.notice)
	}
	_, cmd = a.Update(struct{}{})
	applyPluginEvents(t, a, cmd)
	if a.notice != "decorated" {
		t.Fatal("decoration caused a second text event")
	}
}

// TestBufferEventsRetainLatestIndexWhenWorkerQueueIsFull prevents a lost final edit under load.
func TestBufferEventsRetainLatestIndexWhenWorkerQueueIsFull(t *testing.T) {
	a := pluginFixture(t, `wraith.commands.create("block",function() while true do end end)
wraith.commands.create("noop",function() end)
wraith.events.on("buffer_changed",function(ctx)
 local b=wraith.buffers.get(wraith.buffers.native("prompt"))
 wraith.notify(b.lines[1])
end)`)
	a.syncNativeSurfaces()
	var blocked <-chan plugin.Result
	var err error
	for _, command := range a.pluginManifest.Commands {
		if command.Name == "block" {
			blocked, err = a.plugins.Submit(command.Callback, "", nil, a.Config)
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	var queued []<-chan plugin.Result
	full := false
	for range 200 {
		result, e := a.plugins.Submit("", "plugin.test.noop", nil, a.Config)
		if errors.Is(e, plugin.ErrQueueFull) {
			full = true
			break
		}
		if e != nil {
			t.Fatal(e)
		}
		queued = append(queued, result)
	}
	if !full {
		t.Fatal("queue did not fill")
	}
	a.Prompt.Set("final edit")
	a.Update(struct{}{})
	if !a.bufferEventPending {
		t.Fatal("full queue dropped pending text event")
	}
	select {
	case <-blocked:
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not recover")
	}
	for _, result := range queued {
		select {
		case <-result:
		case <-time.After(time.Second):
			t.Fatal("queue did not drain")
		}
	}
	_, cmd := a.Update(struct{}{})
	applyPluginEvents(t, a, cmd)
	if a.bufferEventPending || a.notice != "final edit" {
		t.Fatalf("latest state not delivered: %s", a.notice)
	}
}

// TestLargePromptKeepsPluginCallbacksAvailable checks bounded mirrors without changing the actual draft.
func TestLargePromptKeepsPluginCallbacksAvailable(t *testing.T) {
	a := pluginFixture(t, `wraith.commands.create("inspect",function(ctx)
 assert(ctx.prompt_truncated and #ctx.prompt<=32768)
 assert(wraith.buffers.get(wraith.buffers.native("prompt")).truncated)
 wraith.notify("available")
end)`)
	draft := strings.Repeat("界", 400000)
	a.Prompt.Set(draft)
	a.Update(a.runCommand("inspect")())
	if a.notice != "available" || a.Prompt.String() != draft {
		t.Fatalf("large draft callback failed: %s", a.notice)
	}
}
