package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// drainManagerEvents publishes each registration before delivering its queued callbacks.
func drainManagerEvents(t *testing.T, r *Runtime, result Result) []string {
	t.Helper()
	var notices []string
	count := 0
	var visit func(Result)
	visit = func(current Result) {
		if count >= 64 {
			t.Fatal("manager event chain exceeded 64 results")
		}
		count++
		if current.Err != nil {
			t.Fatal(current.Err)
		}
		if current.Manifest != nil {
			if err := r.Acknowledge(current.Revision, true); err != nil {
				t.Fatal(err)
			}
		}

		for _, effect := range current.Effects {
			var pending <-chan Result
			var err error
			switch effect.Name {
			case "notify":
				notices = append(notices, effect.Argument)
				continue
			case "schedule.callback":
				pending, err = r.Submit(effect.Argument, "", nil, r.Manifest().Config)
			case "event.dispatch":
				delivery := effect.Data.(EventDelivery)
				pending, err = r.SubmitEvent(effect.Argument, delivery.Listeners, map[string]any{"data": delivery.Data}, r.Manifest().Config, "")
			default:
				t.Fatalf("unexpected manager effect: %+v", effect)
			}
			if err != nil {
				t.Fatal(err)
			}
			visit(receiveResult(t, pending))
		}
	}
	visit(result)
	return notices
}

// TestManagerReadyReplayCoversSourceAndSetup checks real files across startup and late activation.
func TestManagerReadyReplayCoversSourceAndSetup(t *testing.T) {
	for _, trigger := range []string{"before ready", "manual", "eager", "ready", "test.load"} {
		t.Run(trigger, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Mkdir(filepath.Join(root, "lua"), 0o700); err != nil {
				t.Fatal(err)
			}
			source := `wraith.events.on("ready", function()
 wraith.commands.create("source.ready", function() end)
 wraith.notify("source ready")
end)
wraith.events.on("test.load", function(ctx) wraith.notify("source:" .. ctx.data.value) end)
return { setup = function()
 wraith.events.on("ready", function() wraith.notify("setup ready") end)
 wraith.events.on("test.load", function(ctx) wraith.notify("setup:" .. ctx.data.value) end)
end }
`
			if err := os.WriteFile(filepath.Join(root, "lua", "ready_fixture.lua"), []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			policy := "lazy = true"
			switch trigger {
			case "eager":
				policy = "lazy = false"
			case "ready", "test.load":
				policy = fmt.Sprintf("event = %q", trigger)
			}
			r := openManagerRuntime(t, fmt.Sprintf(`local manager = require("shroud")
manager.setup({ { name = "ready-fixture", dir = %q, opts = {}, %s } })
wraith.commands.create("load", function() manager.load("ready-fixture") end)
`, root, policy))
			invoke := func(event string) Result {
				t.Helper()
				var callback string
				if event == "" {
					for _, command := range r.Manifest().Commands {
						if command.Name == "load" {
							callback = command.Callback
						}
					}
					if callback == "" {
						t.Fatal("load command missing")
					}
				}
				pending, err := r.Submit(callback, event, map[string]any{"data": map[string]any{"value": "original"}}, r.Manifest().Config)
				if err != nil {
					t.Fatal(err)
				}
				return receiveResult(t, pending)
			}

			var notices []string
			if trigger == "before ready" {
				if got := drainManagerEvents(t, r, invoke("")); len(got) != 0 {
					t.Fatalf("ready delivered before initial broadcast: %v", got)
				}
			}
			notices = append(notices, drainManagerEvents(t, r, invoke("ready"))...)
			if trigger == "manual" {
				notices = append(notices, drainManagerEvents(t, r, invoke(""))...)
			}
			if trigger == "test.load" {
				notices = append(notices, drainManagerEvents(t, r, invoke("test.load"))...)
			}

			want := []string{"setup ready", "source ready"}
			if trigger == "test.load" {
				want = append(want, "setup:original", "source:original")
			}
			slices.Sort(notices)
			slices.Sort(want)
			if !slices.Equal(notices, want) {
				t.Fatalf("activation delivery: got %v, want %v", notices, want)
			}
			if got := drainManagerEvents(t, r, invoke("")); len(got) != 0 {
				t.Fatalf("cached activation repeated ready: %v", got)
			}
		})
	}
}
