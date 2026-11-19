package ui

import (
	"strings"
	"testing"
)

func TestPluginTargetReceiptTracksFullInputWithoutRegistryChanges(t *testing.T) {
	for _, test := range []struct {
		name    string
		mutate  func(*App)
		changed bool
	}{
		{"same state", func(*App) {}, false},
		{"theme", func(a *App) { a.Config.Theme = "nord" }, false},
		{"keymap publication", func(a *App) { a.Config.Keymaps = nil }, false},
		{"draft tail", func(a *App) { a.Prompt.Set(strings.Repeat("x", 40<<10) + "changed") }, true},
		{"cursor", func(a *App) { a.Prompt.Pos = 1 }, true},
		{"focus", func(a *App) { a.focus = "chat" }, true},
		{"session", func(a *App) { a.Session.ID = "different-session" }, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			a := fixture(t)
			a.Prompt.Set(strings.Repeat("x", 40<<10))
			before := a.pluginSnapshot("", "", nil)
			test.mutate(a)
			after := a.pluginSnapshot("", "", nil)
			token, ok := before["target"].(string)
			if !ok || len(token) != 64 {
				t.Fatalf("invalid receipt: %#v", before["target"])
			}
			if changed := token != after["target"]; changed != test.changed {
				t.Fatalf("receipt changed=%v, want %v", changed, test.changed)
			}
			if test.name == "draft tail" && before["prompt"] != after["prompt"] {
				t.Fatal("test failed to exercise a hidden draft suffix")
			}
		})
	}
}

func TestDeferredKeyTargetRejectsInterveningInput(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*App)
		want   string
	}{
		{"unchanged", nil, "delivered once"},
		{"focus changed", func(a *App) { a.focus = "chat" }, ""},
		{"draft changed", func(a *App) { a.Prompt.Set("new draft") }, "new draft"},
	} {
		t.Run(test.name, func(t *testing.T) {
			a := pluginFixture(t, `wraith.commands.create("defer",function(original)
 wraith.commands.create("published",function() end)
 wraith.schedule(function(current)
  if original.target~=current.target then wraith.notify("stale target"); return end
  wraith.prompt.set("delivered once")
 end)
end)`)
			_, next := a.Update(a.runCommand("defer")())
			if next == nil {
				t.Fatal("first input was not scheduled")
			}
			if test.mutate != nil {
				test.mutate(a)
			}
			scheduled, ok := next().(pluginScheduledMsg)
			if !ok {
				t.Fatal("registration did not defer key delivery")
			}
			applyPluginEvents(t, a, a.runPluginSchedule(scheduled))
			if a.Prompt.String() != test.want {
				t.Fatalf("deferred result = %q, want %q; %s", a.Prompt.String(), test.want, a.notice)
			}
			if test.mutate != nil && a.notice != "stale target" {
				t.Fatalf("stale handoff was not reported: %s", a.notice)
			}
		})
	}
}
