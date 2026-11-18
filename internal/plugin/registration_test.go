package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/erdajt/wraith/internal/config"
)

func TestLuaRegistersCallbacksAndRemovesOverrides(t *testing.T) {
	dir := t.TempDir()
	source := `
wraith.keymap.set("chat.normal", "<C-h>", "window.left")
wraith.keymap.del("chat.normal", "<C-h>")
wraith.commands.create("scratch", function(ctx) assert(ctx.argument == "hello") end, "check argument")
wraith.keymap.set("chat.normal", "gs", "command.scratch")
wraith.keymap.set("prompt.normal", "x", function() end)
local event = wraith.events.on("ready", function() error("removed") end)
wraith.events.off(event)
`
	if err := os.WriteFile(filepath.Join(dir, "init.lua"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := Open(t.Context(), dir, config.Default(), []string{"window.left"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	m := r.Manifest()
	if len(m.Config.Keymaps) != 2 || len(m.Commands) != 1 || len(m.Callbacks) != 3 {
		t.Fatalf("unexpected registrations: %+v", m)
	}
	for _, call := range [][2]string{{m.Commands[0].Callback, ""}, {"", "ready"}} {
		result, err := r.Submit(call[0], call[1], map[string]any{"argument": "hello"}, m.Config)
		if err != nil {
			t.Fatal(err)
		}
		if got := <-result; got.Err != nil {
			t.Fatal(got.Err)
		}
	}
}
