package plugin

import (
	"encoding/json"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

const annotationInventory = `local function inventory()
 local found, visited = {}, {}
 local function walk(path, value)
  local kind = type(value)
  found[path] = kind
  if kind == "number" or kind == "boolean" or kind == "string" then
   found[path] = kind .. ":" .. tostring(value)
  elseif kind == "table" then
   assert(not visited[value], "cyclic export: " .. path)
   visited[value] = true
   for key, child in pairs(value) do
    assert(type(key) == "string", "nonstring export: " .. path)
    walk(path .. "." .. key, child)
   end
  end
 end
 walk("wraith", wraith)
 walk("manager", require("wraith.manager"))
 return found
end
`

// TestLuaAnnotationsMatchRuntime compares executable declarations with the actual public exports.
func TestLuaAnnotationsMatchRuntime(t *testing.T) {
	runtime := openTestRuntime(t, annotationInventory+`print(wraith.json.encode(inventory()))`)
	diagnostics := runtime.Diagnostics()
	if len(diagnostics) != 1 {
		t.Fatalf("runtime inventory diagnostics: %v", diagnostics)
	}
	actual := map[string]string{}
	if err := json.Unmarshal([]byte(diagnostics[0]), &actual); err != nil {
		t.Fatal(err)
	}

	root, err := filepath.Abs(filepath.Join("..", "..", "docs", "lua", "types"))
	if err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(root, "*.lua"))
	if err != nil || len(files) == 0 {
		t.Fatalf("annotation library is missing: %v", err)
	}
	state := lua.NewState()
	defer state.Close()
	if err := state.DoString(fmt.Sprintf(`package.path = %q`, filepath.Join(root, "?.lua"))); err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		if err := state.DoFile(path); err != nil {
			t.Fatalf("load annotation %s: %v", filepath.Base(path), err)
		}
	}
	if err := state.DoString(annotationInventory + `return inventory()`); err != nil {
		t.Fatal(err)
	}
	declared := map[string]string{}
	state.CheckTable(-1).ForEach(func(key, value lua.LValue) {
		declared[key.String()] = value.String()
	})

	paths := slices.Sorted(maps.Keys(actual))
	for _, path := range paths {
		if got := declared[path]; got != actual[path] {
			t.Errorf("annotation %s = %q, runtime = %q", path, got, actual[path])
		}
	}
	for _, path := range slices.Sorted(maps.Keys(declared)) {
		if _, exists := actual[path]; !exists {
			t.Errorf("annotation %s has no runtime export", path)
		}
	}
}
