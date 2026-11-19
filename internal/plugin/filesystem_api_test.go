package plugin

import (
	"os"
	"path/filepath"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func filesystemFixture(t *testing.T) *lua.LState {
	t.Helper()
	L := lua.NewState()
	t.Cleanup(L.Close)
	api := L.NewTable()
	L.SetGlobal("wraith", api)
	(&engine{L: L}).installFilesystem(api)
	L.SetGlobal("root", lua.LString(t.TempDir()))
	return L
}

func TestLuaFilesystemCreatesAndMovesPackageDirectory(t *testing.T) {
	L := filesystemFixture(t)
	source := `local fs = wraith.fs
fs.mkdir(root .. "/staged/lua", {parents=true})
local content = string.char(255,0,195,169) .. "\r\n"
assert(fs.write(root .. "/staged/lua/init.lua", content).bytes == #content)
assert(fs.read(root .. "/staged/lua/init.lua").data == content)
assert(fs.stat(root .. "/staged/lua/init.lua").mode == 384)
assert(fs.list(root .. "/staged").entries[1].name == "lua")
fs.rename(root .. "/staged", root .. "/installed")
assert(fs.stat(root .. "/installed").type == "directory")
assert(not pcall(fs.remove, root .. "/installed"))
fs.remove(root .. "/installed", {recursive=true})
assert(not pcall(fs.stat, root .. "/installed"))`
	if err := L.DoString(source); err != nil {
		t.Fatal(err)
	}
}

func TestLuaFilesystemRejectsInvalidOptionsBeforeWriting(t *testing.T) {
	L := filesystemFixture(t)
	for _, expression := range []string{
		`wraith.fs.write(root .. "/bad", "data", {mode=4096})`,
		`wraith.fs.write(root .. "/bad", "data", {overwrite="yes"})`,
		`wraith.fs.write(root .. "/bad", "data", {parents=true})`,
		`wraith.fs.read(root, {max_bytes=0})`,
		`wraith.fs.read(root, {max_bytes=5000000})`,
		`wraith.fs.read(root, {offset=-1})`,
		`wraith.fs.read(root, {offset=0/0})`,
		`wraith.fs.read(root, {["max_bytes offset"]=1})`,
		`wraith.fs.list(root, {limit=1025})`,
		`wraith.fs.write("", "data")`,
		`wraith.fs.write(root .. string.char(0), "data")`,
		`wraith.fs.write(root .. "/bad", "data", function() end)`,
	} {
		if err := L.DoString(expression); err == nil {
			t.Errorf("accepted invalid filesystem call: %s", expression)
		}
	}
	path := filepath.Join(L.GetGlobal("root").String(), "bad")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("invalid write changed filesystem: %v", err)
	}
}

func TestLuaFilesystemDoesNotFollowSymlinksDuringRemoval(t *testing.T) {
	L := filesystemFixture(t)
	dir := L.GetGlobal("root").String()
	if err := os.WriteFile(filepath.Join(dir, "target"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "target"), filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	if err := L.DoString(`assert(wraith.fs.stat(root .. "/link").type == "symlink")
assert(wraith.fs.stat(root .. "/link", {follow=true}).type == "file")
wraith.fs.remove(root .. "/link", {recursive=true})
assert(wraith.fs.read(root .. "/target").data == "keep")`); err != nil {
		t.Fatal(err)
	}
}
