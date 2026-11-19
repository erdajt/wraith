package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/erdajt/wraith/internal/config"
)

func TestLuaViewAPIsShareTextAndKeepIndependentLayout(t *testing.T) {
	directory := t.TempDir()
	source := `
local b = wraith.buffers.create({lines={"one", "two"}})
local left = wraith.windows.open(b, {position="left", cursor={line=1,column=2}})
local right = wraith.windows.open(b, {position="right"})
assert(wraith.windows.configure(right, {title="details", cursor={line=0,column=1}}) == 2)
assert(wraith.windows.get(left).cursor.line == 1)
assert(wraith.windows.get(right).buffer_id == b)
assert(not pcall(wraith.windows.configure, right, {width=-1}))
assert(not pcall(wraith.windows.open, b, {kind="float"}))
assert(#wraith.windows.list() == 2)
wraith.windows.focus(left)
wraith.windows.close(right)
assert(#wraith.windows.list() == 1 and #wraith.buffers.list() == 1)
local header = wraith.regions.create({slot="header",lines={"status"}})
assert(wraith.regions.set(header, {lines={"next"}}) == 2)
assert(wraith.regions.get(header).slot == "header")
assert(not pcall(wraith.regions.create, {slot="sidebar",width=121}))
assert(not pcall(wraith.regions.create, {slot="prompt",lines={"1","2","3","4","5"}}))
assert(#wraith.regions.list() == 1)
assert(wraith.ui.decode("a"..string.char(27).."b\n") == "ab\n")
assert(type(wraith.ui.select) == "function" and type(wraith.ui.input) == "function")
`
	if err := os.WriteFile(filepath.Join(directory, "init.lua"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	runtime, err := Open(t.Context(), directory, config.Default(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	for _, mutation := range runtime.Manifest().Surface {
		if _, err := runtime.Surfaces().Apply(mutation); err != nil {
			t.Fatal(err)
		}
	}
	index := runtime.Surfaces().Index()
	if len(index.Windows) != 1 || len(index.Regions) != 1 || len(index.Buffers) != 1 {
		t.Fatalf("committed layout: %+v", index)
	}
	if len(runtime.Surfaces().reserved) != 0 {
		t.Fatalf("failed creates retained reservations: %+v", runtime.Surfaces().reserved)
	}
}

func TestLuaWindowsPreserveNativeAndOwnedHandles(t *testing.T) {
	r := openTestRuntime(t, `
local b = wraith.buffers.create({lines={"owned"}})
local w = wraith.windows.open(b)
wraith.commands.create("mixed",function()
 local windows = wraith.windows.list()
 assert(#windows == 2 and windows[1].id == 7 and windows[2].id == w)
 wraith.windows.split("vertical")
 wraith.windows.focus(windows[1].id)
 wraith.windows.close()
 wraith.windows.focus(w)
end)
`)
	manifest := r.Manifest()
	for _, mutation := range manifest.Surface {
		if _, err := r.Surfaces().Apply(mutation); err != nil {
			t.Fatal(err)
		}
	}
	pending, err := r.Submit(manifest.Commands[0].Callback, "", map[string]any{"windows": []any{map[string]any{"id": 7}}}, manifest.Config)
	if err != nil {
		t.Fatal(err)
	}
	got := receiveResult(t, pending)
	if got.Err != nil || len(got.Effects) != 4 || got.Effects[0].Name != "window.split_vertical" || got.Effects[1].Argument != "7" || got.Effects[2].Argument != "0" || got.Effects[3].Name != "surface.apply" {
		t.Fatalf("mixed window actions: %+v", got)
	}
}
