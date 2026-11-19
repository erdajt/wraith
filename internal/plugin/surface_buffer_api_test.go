package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/erdajt/wraith/internal/config"
)

func TestLuaBufferAPIStagesTypedTextAndRejectsInvalidOptions(t *testing.T) {
	directory := t.TempDir()
	source := `
assert(#wraith.buffers.list() == 0 and #wraith.regions.list() == 0)
local b = wraith.buffers.create({name="notes", editable=true, lines={"écho", "two"}})
assert(wraith.buffers.get(b).text_revision == 1)
assert(#wraith.buffers.get(b).keymaps == 0 and #wraith.buffers.get(b).spans == 0)
assert(#wraith.buffers.get_lines(b,0,0).lines == 0)
assert(wraith.buffers.set_lines(b, 1, -1, {"next"}) == 2)
local page = wraith.buffers.get_lines(b, 0, -1)
assert(page.lines[1] == "écho" and page.lines[2] == "next" and page.next == 2)
assert(not pcall(wraith.buffers.set_lines, b, 0, 1, {"stale"}, 1))
assert(not pcall(wraith.buffers.get_lines, b, 0.5, 1))
assert(not pcall(wraith.buffers.set_lines, b, 0, "1", {"wrong"}))
assert(not pcall(wraith.buffers.create, {lines={"text"}, mystery=true}))
assert(not pcall(wraith.buffers.create, {lines={string.char(255)}}))
assert(not pcall(wraith.buffers.set_lines, b, 0, 1, {string.char(255)}))
assert(wraith.ui.decode(string.char(255)) == "�")
local ok,err = pcall(wraith.buffers.create,{[string.rep("x",1024*1024+1)]=true})
assert(not ok and tostring(err):find("1 mib",1,true))
assert(wraith.buffers.set_spans(b, {{start={line=0,column=0}, ["end"]={line=0,column=1}, style={foreground="accent"}}}) == 3)
assert(wraith.buffers.set_spans(b, {}) == 4)
assert(wraith.buffers.native("prompt") == nil)
assert(#wraith.buffers.list() == 1)
`
	if err := os.WriteFile(filepath.Join(directory, "init.lua"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	runtime, err := Open(t.Context(), directory, config.Default(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	manifest := runtime.Manifest()
	if len(manifest.Surface) != 4 || len(runtime.Surfaces().Index().Buffers) != 0 {
		t.Fatalf("startup mutations: %+v", manifest.Surface)
	}
	for _, mutation := range manifest.Surface {
		if _, err := runtime.Surfaces().Apply(mutation); err != nil {
			t.Fatal(err)
		}
	}
	buffer, _ := runtime.Surfaces().Buffer(manifest.Surface[0].ID)
	if buffer.Lines[1] != "next" || buffer.Revision != 4 || buffer.TextRevision != 2 {
		t.Fatalf("committed buffer: %+v", buffer)
	}
}
