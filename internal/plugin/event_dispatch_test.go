package plugin

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/erdajt/wraith/internal/config"
	lua "github.com/yuin/gopher-lua"
)

func TestObserverFailurePreservesSuccessfulSubscribers(t *testing.T) {
	r := openTestRuntime(t, `wraith.events.on("response_finished", function()
 wraith.commands.create("kept", function() wraith.notify("alive") end)
 wraith.notify("first")
end)
wraith.events.on("response_finished", function()
 wraith.commands.create("discarded", function() end)
 wraith.notify("discarded")
 error("observer failed")
end)
wraith.events.on("response_finished", function() wraith.notify("last") end)`)
	m := r.Manifest()
	pending, err := r.Submit("", "response_finished", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	got := receiveResult(t, pending)
	if got.Err != nil || len(got.Effects) != 2 || got.Effects[0].Argument != "first" || got.Effects[1].Argument != "last" {
		t.Fatalf("subscriber effects: %+v", got)
	}
	if got.Manifest == nil || len(got.Manifest.Commands) != 1 || got.Manifest.Commands[0].Name != "kept" {
		t.Fatalf("subscriber registrations: %+v", got.Manifest)
	}
	if err := r.Acknowledge(got.Revision, true); err != nil {
		t.Fatal(err)
	}
	select {
	case notice := <-r.Notices():
		if !strings.Contains(notice, "event response_finished observer config") || !strings.Contains(notice, "observer failed") {
			t.Fatalf("missing observer diagnostic: %s", notice)
		}
	case <-time.After(time.Second):
		t.Fatal("observer failure was not reported")
	}
}

func TestObserverBatchDoesNotShareCommandDeadline(t *testing.T) {
	r := openTestRuntime(t, `for i = 1, 6 do
 wraith.events.on("response_finished", function() while true do end end)
end
wraith.events.on("response_finished", function() wraith.notify("all observers reached") end)`)
	pending, err := r.Submit("", "response_finished", nil, r.Manifest().Config)
	if err != nil {
		t.Fatal(err)
	}
	got := receiveResult(t, pending)
	if got.Err != nil || len(got.Effects) != 1 || got.Effects[0].Argument != "all observers reached" || len(r.Diagnostics()) != 6 {
		t.Fatalf("observer batch shared a deadline: %+v, diagnostics: %v", got, r.Diagnostics())
	}
}

func TestInvalidObserverMappingDoesNotSuppressHealthySubscriber(t *testing.T) {
	r := openTestRuntime(t, `wraith.events.on("response_finished", function()
 wraith.keymap.set("prompt.normal", "x", "missing.action")
 wraith.notify("discarded")
end)
wraith.events.on("response_finished", function() wraith.notify("healthy") end)`)
	pending, err := r.Submit("", "response_finished", nil, r.Manifest().Config)
	if err != nil {
		t.Fatal(err)
	}
	got := receiveResult(t, pending)
	if got.Err != nil || got.Manifest != nil || len(got.Effects) != 1 || got.Effects[0].Argument != "healthy" {
		t.Fatalf("invalid observer mapping affected healthy subscriber: %+v", got)
	}
	diagnostics := r.Diagnostics()
	if len(diagnostics) != 1 || !strings.Contains(diagnostics[0], "unknown action missing.action") {
		t.Fatalf("missing mapping failure diagnostic: %v", diagnostics)
	}
}

func TestParentCancellationStopsObserverBatch(t *testing.T) {
	dir := t.TempDir()
	source := `wraith.events.on("response_finished", function()
 wraith.notify("discarded")
 cancel_dispatch()
end)
wraith.events.on("response_finished", function() later_ran = true end)`
	if err := os.WriteFile(filepath.Join(dir, "init.lua"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	e, err := load(t.Context(), dir, config.Default(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer e.L.Close()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	e.L.SetGlobal("cancel_dispatch", e.L.NewFunction(func(*lua.LState) int {
		cancel()
		return 0
	}))

	got := e.invoke(ctx, request{event: "response_finished", data: []byte("null"), config: e.manifest.Config})
	if !errors.Is(got.Err, context.Canceled) || len(got.Effects) != 0 || e.L.GetGlobal("later_ran") != lua.LNil {
		t.Fatalf("parent cancellation did not stop the batch: %+v", got)
	}
	if len(e.observerErrors) != 0 {
		t.Fatalf("parent cancellation reported an observer failure: %v", e.observerErrors)
	}
}

func TestObserverTimeoutDoesNotCancelLaterCallbacks(t *testing.T) {
	r := openTestRuntime(t, `package.preload.slow = function()
 return {setup = function()
  wraith.events.on("response_finished", function() while true do end end)
 end}
end
wraith.plugins.load("slow")
wraith.events.on("response_finished", function() wraith.notify("after timeout") end)
wraith.commands.create("check", function() wraith.notify("command works") end)`)
	m := r.Manifest()
	pending, err := r.Submit("", "response_finished", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	got := receiveResult(t, pending)
	if got.Err != nil || len(got.Effects) != 1 || got.Effects[0].Argument != "after timeout" {
		t.Fatalf("timeout suppressed later subscriber: %+v", got)
	}
	select {
	case notice := <-r.Notices():
		if !strings.Contains(notice, "observer plugin:slow:") || !strings.Contains(notice, "context deadline exceeded") {
			t.Fatalf("missing owner or timeout: %s", notice)
		}
	case <-time.After(time.Second):
		t.Fatal("observer timeout was not reported")
	}
	pending, err = r.Submit(m.Commands[0].Callback, "", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	got = receiveResult(t, pending)
	if got.Err != nil || len(got.Effects) != 1 || got.Effects[0].Argument != "command works" {
		t.Fatalf("observer context leaked into command: %+v", got)
	}
}

func TestObserverMembershipUsesDispatchSnapshot(t *testing.T) {
	r := openTestRuntime(t, `local self, removed
self = wraith.events.on("response_finished", function()
 wraith.events.off(self)
 wraith.events.off(removed)
 wraith.events.on("response_finished", function() wraith.notify("new") end)
 wraith.notify("original")
end)
removed = wraith.events.on("response_finished", function() error("removed observer ran") end)`)
	m := r.Manifest()
	pending, err := r.Submit("", "response_finished", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	got := receiveResult(t, pending)
	if got.Err != nil || got.Manifest == nil || len(got.Effects) != 1 || got.Effects[0].Argument != "original" {
		t.Fatalf("first dispatch: %+v", got)
	}
	if err := r.Acknowledge(got.Revision, true); err != nil {
		t.Fatal(err)
	}
	m = r.Manifest()
	pending, err = r.Submit("", "response_finished", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	got = receiveResult(t, pending)
	if got.Err != nil || len(got.Effects) != 1 || got.Effects[0].Argument != "new" || len(r.Diagnostics()) != 0 {
		t.Fatalf("next dispatch: %+v, diagnostics: %v", got, r.Diagnostics())
	}
}
