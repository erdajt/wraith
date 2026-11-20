package plugin

import (
	"slices"
	"strings"
	"testing"
)

// TestManagerContinuationRejectedRegistrationDoesNotQueue keeps rejected receipt creation outside live listeners.
func TestManagerContinuationRejectedRegistrationDoesNotQueue(t *testing.T) {
	r := openManagerRuntime(t, `
local queue = require("wraith.manager.install_continuations")
wraith.commands.create("add", function()
	queue.add("operation", function()
		wraith.notify("unexpected")
	end)
end)
wraith.commands.create("wake", function()
	queue.wake("operation")
end)
wraith.commands.create("inspect", function()
	assert(#queue.pending() == 0)
end)
`)
	result := submitManagerCommand(t, r, "add", "")
	if result.Err != nil || result.Manifest == nil {
		t.Fatalf("receipt registration: %+v", result)
	}
	if err := r.Acknowledge(result.Revision, false); err != nil {
		t.Fatal(err)
	}
	managerCommand(t, r, "inspect", "")
	result = submitManagerCommand(t, r, "wake", "")
	if result.Err != nil || len(result.Effects) != 0 {
		t.Fatalf("rejected receipt produced delivery: %+v", result)
	}
}

// TestManagerContinuationFailedAttemptCanBeWokenAgain retries through actual event, schedule and publication boundaries.
func TestManagerContinuationFailedAttemptCanBeWokenAgain(t *testing.T) {
	for _, mode := range []string{"error", "deadline", "rejection", "discard"} {
		t.Run(mode, func(t *testing.T) {
			r := openManagerRuntime(t, `
local queue = require("wraith.manager.install_continuations")
queue.add("operation", function(ctx)
	wraith.commands.create("partial", function() end)
	if ctx.failure == "error" then
		error("attempt failed")
	end
	while ctx.failure == "deadline" do
	end
	wraith.notify("attempt finished")
end)
wraith.commands.create("wake", function()
	queue.wake("operation")
end)
wraith.commands.create("inspect", function(ctx)
	assert(#queue.pending() == tonumber(ctx.argument))
end, "inspect", { arguments = true })
`)
			for attempt := range 2 {
				woken := submitManagerCommand(t, r, "wake", "")
				if woken.Err != nil || len(woken.Effects) != 1 {
					t.Fatalf("wake: %+v", woken)
				}
				observed := deliverManagerEvent(t, r, woken.Effects[0])
				if observed.Err != nil || observed.Manifest != nil || len(observed.Effects) != 1 || observed.Effects[0].Name != "schedule.callback" {
					t.Fatalf("observer ran action or failed to schedule: %+v", observed)
				}
				callback := observed.Effects[0].Argument
				if attempt == 0 && mode == "discard" {
					r.DiscardScheduled([]string{callback})
				}
				failure := ""
				if attempt == 0 {
					failure = mode
				}
				pending, err := r.Submit(callback, "", map[string]any{"failure": failure}, r.Manifest().Config)
				if err != nil {
					t.Fatal(err)
				}
				result := receiveResult(t, pending)
				if attempt == 0 {
					switch mode {
					case "error", "deadline":
						message := "attempt failed"
						if mode == "deadline" {
							message = "deadline"
						}
						if result.Err == nil || !strings.Contains(result.Err.Error(), message) || result.Manifest != nil || len(result.Effects) != 0 {
							t.Fatalf("failed attempt escaped rollback: %+v", result)
						}
					case "rejection":
						if result.Err != nil || result.Manifest == nil {
							t.Fatalf("attempt publication: %+v", result)
						}
						if err := r.Acknowledge(result.Revision, false); err != nil {
							t.Fatal(err)
						}
					case "discard":
						if result.Err != nil || result.Manifest != nil || len(result.Effects) != 0 {
							t.Fatalf("discarded attempt ran: %+v", result)
						}
					}
					if slices.ContainsFunc(r.Manifest().Commands, func(command Command) bool { return command.Name == "partial" }) {
						t.Fatal("unaccepted attempt retained partial registration")
					}
					managerCommand(t, r, "inspect", "1")
					continue
				}

				if result.Err != nil || result.Manifest == nil || len(result.Effects) != 1 || result.Effects[0].Argument != "attempt finished" {
					t.Fatalf("explicit retry failed: %+v", result)
				}
				if err := r.Acknowledge(result.Revision, true); err != nil {
					t.Fatal(err)
				}
				managerCommand(t, r, "inspect", "0")
				if !slices.ContainsFunc(r.Manifest().Commands, func(command Command) bool { return command.Name == "partial" }) {
					t.Fatal("accepted attempt lost its registration")
				}
			}
		})
	}
}
