package plugin

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

//go:embed lua/wraith/manager/*.lua
var bundledLua embed.FS

// installBundledModules publishes shipped Lua through the ordinary require cache and loader.
func (e *engine) installBundledModules() error {
	preload := e.L.GetField(e.L.GetGlobal("package"), "preload").(*lua.LTable)
	return fs.WalkDir(bundledLua, "lua", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".lua") {
			return nil
		}
		source, err := bundledLua.ReadFile(path)
		if err != nil {
			return fmt.Errorf("bundled module %s: %w", path, err)
		}
		name := strings.TrimSuffix(strings.TrimPrefix(path, "lua/"), ".lua")
		name = strings.ReplaceAll(strings.TrimSuffix(name, "/init"), "/", ".")
		preload.RawSetString(name, e.L.NewFunction(func(L *lua.LState) int {
			chunk, err := L.Load(bytes.NewReader(source), "@builtin/"+path)
			if err != nil {
				L.RaiseError("module %s: %v", name, err)
			}
			if err := L.CallByParam(lua.P{Fn: chunk, NRet: 1, Protect: true}, L.Get(1)); err != nil {
				L.RaiseError("module %s: %v", name, err)
			}
			return 1
		}))
		return nil
	})
}
