package plugin

import (
	"fmt"
	"testing"
)

// TestManagerContinuationCancellationFollowsPublication retains queued attempts only when removal is rejected.
func TestManagerContinuationCancellationFollowsPublication(t *testing.T) {
	for _, operation := range []string{"cancel", "unload"} {
		for _, accepted := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%t", operation, accepted), func(t *testing.T) {
				r := openManagerRuntime(t, `
local queue = require("wraith.manager.install_continuations")
local receipt
package.preload.continuation_owner = function()
	return {
		setup = function()
			receipt = queue.add("operation", function()
				wraith.notify("attempt finished")
			end)
		end,
	}
end
wraith.plugins.load("continuation_owner")
wraith.commands.create("wake", function()
	queue.wake("operation")
end)
wraith.commands.create("cancel", function()
	assert(queue.cancel(receipt))
end)
wraith.commands.create("unload", function()
	wraith.plugins.unload("continuation_owner")
end)
wraith.commands.create("inspect", function(ctx)
	assert(#queue.pending() == tonumber(ctx.argument))
end, "inspect", { arguments = true })
`)
				woken := submitManagerCommand(t, r, "wake", "")
				if woken.Err != nil || len(woken.Effects) != 1 {
					t.Fatalf("wake: %+v", woken)
				}
				observed := deliverManagerEvent(t, r, woken.Effects[0])
				if observed.Err != nil || len(observed.Effects) != 1 || observed.Effects[0].Name != "schedule.callback" {
					t.Fatalf("pending attempt: %+v", observed)
				}
				removed := submitManagerCommand(t, r, operation, "")
				if removed.Err != nil || removed.Manifest == nil {
					t.Fatalf("receipt removal: %+v", removed)
				}
				if err := r.Acknowledge(removed.Revision, accepted); err != nil {
					t.Fatal(err)
				}
				count := "1"
				if accepted {
					count = "0"
				}
				managerCommand(t, r, "inspect", count)
				pending, err := r.Submit(observed.Effects[0].Argument, "", nil, r.Manifest().Config)
				if err != nil {
					t.Fatal(err)
				}
				result := receiveResult(t, pending)
				if accepted {
					if result.Err != nil || result.Manifest != nil || len(result.Effects) != 0 {
						t.Fatalf("accepted removal left runnable work: %+v", result)
					}
				} else {
					if result.Err != nil || result.Manifest == nil || len(result.Effects) != 1 || result.Effects[0].Argument != "attempt finished" {
						t.Fatalf("rejected removal lost runnable work: %+v", result)
					}
					if err := r.Acknowledge(result.Revision, true); err != nil {
						t.Fatal(err)
					}
				}
				managerCommand(t, r, "inspect", "0")
			})
		}
	}
}
