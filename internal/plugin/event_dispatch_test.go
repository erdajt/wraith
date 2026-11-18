package plugin

import (
	"strings"
	"testing"
	"time"
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
