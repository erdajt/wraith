package plugin

import (
	"strings"
	"testing"
)

// TestReadyReplayRestoresNestedEffects keeps rejected calls from consuming later delivery.
func TestReadyReplayRestoresNestedEffects(t *testing.T) {
	r := openTestRuntime(t, `
local first = wraith.events.on("ready", function() wraith.notify("first") end)
local second = wraith.events.on("ready", function() wraith.notify("second") end)
package.preload.child = function() return {
 setup = function() end,
 replay = function()
  wraith.events.replay_ready({first, second})
  error("nested failure")
 end,
} end
wraith.plugins.load("child")
wraith.commands.create("replay", function()
 wraith.events.replay_ready({first})
 assert(not pcall(wraith.plugins.call, "child", "replay"))
 wraith.events.replay_ready({first, second})
end)
`)
	m := r.Manifest()
	pending, err := r.Submit("", "ready", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	if result := receiveResult(t, pending); result.Err != nil || len(result.Effects) != 2 {
		t.Fatalf("initial ready: %+v", result)
	}
	pending, err = r.Submit(m.Commands[0].Callback, "", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	result := receiveResult(t, pending)
	if result.Err != nil || len(result.Effects) != 2 {
		t.Fatalf("nested replay restoration: %+v", result)
	}
	for index, effect := range result.Effects {
		delivery := effect.Data.(EventDelivery)
		if effect.Name != "event.dispatch" || effect.Argument != "ready" || len(delivery.Listeners) != 1 || delivery.Data != nil {
			t.Fatalf("replay mutated an earlier delivery: %+v", effect)
		}
		pending, err = r.SubmitEvent("ready", delivery.Listeners, nil, m.Config, "")
		if err != nil {
			t.Fatal(err)
		}
		got := receiveResult(t, pending)
		if got.Err != nil || len(got.Effects) != 1 || got.Effects[0].Argument != []string{"first", "second"}[index] {
			t.Fatalf("replay order: %+v", got)
		}
	}
}

// TestReadyReplayRejectedPublicationAllowsRetry preserves catchup after host rejection.
func TestReadyReplayRejectedPublicationAllowsRetry(t *testing.T) {
	r := openTestRuntime(t, `
local listener = wraith.events.on("ready", function() wraith.notify("delivered") end)
wraith.commands.create("replay", function()
 wraith.commands.create("added", function() end)
 wraith.events.replay_ready({listener})
end)
`)
	m := r.Manifest()
	pending, err := r.Submit("", "ready", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	if result := receiveResult(t, pending); result.Err != nil {
		t.Fatal(result.Err)
	}
	for _, accepted := range []bool{false, true} {
		pending, err = r.Submit(m.Commands[0].Callback, "", nil, m.Config)
		if err != nil {
			t.Fatal(err)
		}
		result := receiveResult(t, pending)
		if result.Err != nil || result.Manifest == nil || len(result.Effects) != 1 {
			t.Fatalf("replay publication: %+v", result)
		}
		if err := r.Acknowledge(result.Revision, accepted); err != nil {
			t.Fatal(err)
		}
	}
}

// TestReadyReplayBeforeInitialBroadcastDoesNotDuplicateNewSubscribers covers queued startup work.
func TestReadyReplayBeforeInitialBroadcastDoesNotDuplicateNewSubscribers(t *testing.T) {
	r := openTestRuntime(t, `
wraith.commands.create("early", function()
 local id = wraith.events.on("ready", function() wraith.notify("early") end)
 wraith.events.replay_ready({id})
end)
wraith.events.on("ready", function()
 local id = wraith.events.on("ready", function() wraith.notify("late") end)
 wraith.events.replay_ready({id})
end)
`)
	m := r.Manifest()
	pending, err := r.Submit(m.Commands[0].Callback, "", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	early := receiveResult(t, pending)
	if early.Err != nil || early.Manifest == nil || len(early.Effects) != 0 {
		t.Fatalf("before first broadcast: %+v", early)
	}
	if err := r.Acknowledge(early.Revision, true); err != nil {
		t.Fatal(err)
	}
	pending, err = r.Submit("", "ready", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	ready := receiveResult(t, pending)
	if ready.Err != nil || ready.Manifest == nil || len(ready.Effects) != 2 || ready.Effects[1].Argument != "early" {
		t.Fatalf("initial observer snapshot: %+v", ready)
	}
	if err := r.Acknowledge(ready.Revision, true); err != nil {
		t.Fatal(err)
	}
	delivery := ready.Effects[0].Data.(EventDelivery)
	pending, err = r.SubmitEvent("ready", delivery.Listeners, nil, m.Config, "")
	if err != nil {
		t.Fatal(err)
	}
	got := receiveResult(t, pending)
	if got.Err != nil || len(got.Effects) != 1 || got.Effects[0].Argument != "late" {
		t.Fatalf("new observer catchup: %+v", got)
	}
}

// TestReadyReplayAfterRejectedInitialBroadcast retains the host's started lifecycle phase.
func TestReadyReplayAfterRejectedInitialBroadcast(t *testing.T) {
	r := openTestRuntime(t, `
wraith.events.on("ready", function()
 wraith.commands.create("initial", function() end)
end)
wraith.commands.create("later", function()
 local id = wraith.events.on("ready", function() wraith.notify("later") end)
 wraith.events.replay_ready({id})
end)
`)
	m := r.Manifest()
	pending, err := r.Submit("", "ready", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	initial := receiveResult(t, pending)
	if initial.Err != nil || initial.Manifest == nil {
		t.Fatalf("initial publication: %+v", initial)
	}
	if err := r.Acknowledge(initial.Revision, false); err != nil {
		t.Fatal(err)
	}
	pending, err = r.Submit(m.Commands[0].Callback, "", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	later := receiveResult(t, pending)
	if later.Err != nil || later.Manifest == nil || len(later.Effects) != 1 {
		t.Fatalf("catchup after rejected initial event: %+v", later)
	}
	if err := r.Acknowledge(later.Revision, true); err != nil {
		t.Fatal(err)
	}
	delivery := later.Effects[0].Data.(EventDelivery)
	pending, err = r.SubmitEvent("ready", delivery.Listeners, nil, m.Config, "")
	if err != nil {
		t.Fatal(err)
	}
	if result := receiveResult(t, pending); result.Err != nil || len(result.Effects) != 1 || result.Effects[0].Argument != "later" {
		t.Fatalf("later delivery: %+v", result)
	}
}

// TestReadyReplayDeadlineDoesNotConsumeDelivery discards effects from an interrupted callback.
func TestReadyReplayDeadlineDoesNotConsumeDelivery(t *testing.T) {
	r := openTestRuntime(t, `
local id = wraith.events.on("ready", function() end)
wraith.commands.create("replay", function(ctx)
 wraith.events.replay_ready({id})
 while ctx.block do end
end)
`)
	m := r.Manifest()
	pending, err := r.Submit("", "ready", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	if result := receiveResult(t, pending); result.Err != nil {
		t.Fatal(result.Err)
	}
	for _, blocked := range []bool{true, false} {
		pending, err = r.Submit(m.Commands[0].Callback, "", map[string]any{"block": blocked}, m.Config)
		if err != nil {
			t.Fatal(err)
		}
		result := receiveResult(t, pending)
		if blocked {
			if result.Err == nil || !strings.Contains(result.Err.Error(), "deadline") || len(result.Effects) != 0 || result.Manifest != nil {
				t.Fatalf("interrupted replay escaped: %+v", result)
			}
		} else if result.Err != nil || len(result.Effects) != 1 {
			t.Fatalf("retry after deadline: %+v", result)
		}
	}
}
