package plugin

import (
	"slices"
	"testing"
)

// submitManagerCommand leaves publication and queued effects for the caller to accept or reject.
func submitManagerCommand(t *testing.T, r *Runtime, name, argument string) Result {
	t.Helper()
	for _, command := range r.Manifest().Commands {
		if command.Name != name {
			continue
		}
		pending, err := r.Submit(command.Callback, "", map[string]any{"argument": argument}, r.Manifest().Config)
		if err != nil {
			t.Fatal(err)
		}
		return receiveResult(t, pending)
	}
	t.Fatalf("missing manager fixture command: %s", name)
	return Result{}
}

// deliverManagerEvent sends the exact subscriber snapshot through the host observer path.
func deliverManagerEvent(t *testing.T, r *Runtime, effect Effect) Result {
	t.Helper()
	delivery, ok := effect.Data.(EventDelivery)
	if effect.Name != "event.dispatch" || !ok {
		t.Fatalf("expected targeted event: %+v", effect)
	}
	pending, err := r.SubmitEvent(effect.Argument, delivery.Listeners, map[string]any{"data": delivery.Data}, r.Manifest().Config, "")
	if err != nil {
		t.Fatal(err)
	}
	return receiveResult(t, pending)
}

// TestManagerContinuationConsumesDuplicateWakeupsOnce separates observer delivery from accepted action consumption.
func TestManagerContinuationConsumesDuplicateWakeupsOnce(t *testing.T) {
	r := openManagerRuntime(t, `
local receipts = require("wraith.manager.install_continuations")
local runs = 0
receipts.add("install.first", function()
	runs = runs + 1
	wraith.notify("run:" .. runs)
end)
wraith.commands.create("wake", function(ctx)
	receipts.wake(ctx.argument)
end, "wake", { arguments = true })
wraith.commands.create("pending", function()
	wraith.notify(#receipts.pending())
end)
`)
	wrong := submitManagerCommand(t, r, "wake", "install.other")
	if wrong.Err != nil || wrong.Manifest != nil || len(wrong.Effects) != 1 {
		t.Fatalf("missing wakeup: %+v", wrong)
	}
	ignored := deliverManagerEvent(t, r, wrong.Effects[0])
	if ignored.Err != nil || ignored.Manifest != nil || len(ignored.Effects) != 0 {
		t.Fatalf("unrelated operation ran a receipt: %+v", ignored)
	}

	wake := submitManagerCommand(t, r, "wake", "install.first")
	if wake.Err != nil || wake.Manifest != nil || len(wake.Effects) != 1 {
		t.Fatalf("missing matching wakeup: %+v", wake)
	}
	var attempts []Effect
	for range 2 {
		observed := deliverManagerEvent(t, r, wake.Effects[0])
		if observed.Err != nil || observed.Manifest != nil || len(observed.Effects) != 1 || observed.Effects[0].Name != "schedule.callback" {
			t.Fatalf("observer did not defer action: %+v", observed)
		}
		attempts = append(attempts, observed.Effects[0])
	}
	for index, attempt := range attempts {
		pending, err := r.Submit(attempt.Argument, "", nil, r.Manifest().Config)
		if err != nil {
			t.Fatal(err)
		}
		result := receiveResult(t, pending)
		if index == 0 {
			if result.Err != nil || result.Manifest == nil || len(result.Effects) != 1 || result.Effects[0].Argument != "run:1" {
				t.Fatalf("first attempt did not stage consumption: %+v", result)
			}
			if err := r.Acknowledge(result.Revision, true); err != nil {
				t.Fatal(err)
			}
		} else if result.Err != nil || result.Manifest != nil || len(result.Effects) != 0 {
			t.Fatalf("duplicate attempt consumed twice: %+v", result)
		}
	}
	result := managerCommand(t, r, "pending", "")
	if len(result.Effects) != 1 || result.Effects[0].Argument != "0" {
		t.Fatalf("consumed receipt remained pending: %+v", result)
	}
}

// TestManagerContinuationCapacityReusesPublishedCallbacks bounds live work without retaining retired closures.
func TestManagerContinuationCapacityReusesPublishedCallbacks(t *testing.T) {
	r := openManagerRuntime(t, `
local receipts = require("wraith.manager.install_continuations")
local last
wraith.commands.create("fill", function()
	assert(#receipts.pending() == 0)
	for index = 1, 16 do
		local id = receipts.add("install.batch", function()
			wraith.notify("done")
		end)
		assert(id ~= last)
		last = id
	end
	assert(not pcall(receipts.add, "install.overflow", function()
		error("overflow ran")
	end))
	assert(#receipts.pending() == 16)
	local detached = receipts.pending()
	detached[1] = nil
	assert(#receipts.pending() == 16)
end)
wraith.commands.create("wake", function()
	receipts.wake("install.batch")
end)
`)
	baseline := len(r.Manifest().Callbacks)
	for cycle := range 40 {
		managerCommand(t, r, "fill", "")
		if got := len(r.Manifest().Callbacks); got != baseline+16 {
			t.Fatalf("cycle %d: pending callbacks = %d, want %d", cycle, got, baseline+16)
		}
		notices := drainManagerEvents(t, r, submitManagerCommand(t, r, "wake", ""))
		if len(notices) != 16 || slices.ContainsFunc(notices, func(value string) bool { return value != "done" }) {
			t.Fatalf("cycle %d: batch did not finish: %v", cycle, notices)
		}
		if got := len(r.Manifest().Callbacks); got != baseline {
			t.Fatalf("cycle %d: retired closures retained: got %d want %d", cycle, got, baseline)
		}
	}
}
