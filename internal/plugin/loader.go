package plugin

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// installLoader preserves preload while bounding source reads from explicit package paths.
func (e *engine) installLoader(directory string) error {
	if strings.ContainsAny(directory, ";?\x00") {
		return errors.New("lua configuration path cannot contain semicolon, question mark or nul")
	}
	L := e.L
	pkg := L.GetGlobal("package")
	loaders := L.GetField(pkg, "loaders").(*lua.LTable)
	loaders.RawSetInt(2, L.NewFunction(func(L *lua.LState) int {
		name := strings.ReplaceAll(L.CheckString(1), ".", string(filepath.Separator))
		path, ok := L.GetField(pkg, "path").(lua.LString)
		if !ok {
			L.RaiseError("package.path must be a string")
		}
		var missing []string
		for _, pattern := range strings.Split(string(path), ";") {
			if pattern == "" {
				continue
			}
			filename := strings.ReplaceAll(pattern, "?", name)
			fn, err := loadFile(L, filename)
			if errors.Is(err, os.ErrNotExist) {
				missing = append(missing, fmt.Sprintf("no file %q", filename))
				continue
			}
			if err != nil {
				L.RaiseError("module %s: %v", L.CheckString(1), err)
			}
			L.Push(fn)
			return 1
		}
		L.Push(lua.LString(strings.Join(missing, "\n\t")))
		return 1
	}))
	return nil
}

// installDiagnostics routes print through notices instead of writing into the terminal frame.
func (e *engine) installDiagnostics(api *lua.LTable) {
	L := e.L
	capabilities := L.NewTable()
	for _, name := range []string{"trusted_lua", "runtimepath", "stdpath"} {
		capabilities.RawSetString(name, lua.LBool(name == "trusted_lua"))
	}
	for _, name := range []string{
		"live_registration", "custom_events", "plugin_lifecycle", "scheduling",
		"json", "filesystem", "http", "timers", "managed_resources", "process",
		"buffers", "windows", "regions", "dialogs", "buffer_keymaps",
		"hooks", "providers", "model_tools", "invocations", "workspace_queries",
	} {
		capabilities.RawSetString(name, lua.LTrue)
	}
	capabilities.RawSetString("luajit", lua.LFalse)
	capabilities.RawSetString("native_modules", lua.LFalse)
	api.RawSetString("capabilities", capabilities)
	L.SetGlobal("print", L.NewFunction(func(L *lua.LState) int {
		parts := make([]string, L.GetTop())
		for i := range parts {
			if err := L.CallByParam(lua.P{Fn: L.GetGlobal("tostring"), NRet: 1, Protect: true}, L.Get(i+1)); err != nil {
				L.RaiseError("print: %v", err)
			}
			parts[i] = L.CheckString(-1)
			L.Pop(1)
		}
		message := strings.Join(parts, "\t")
		if len(message) > 64<<10 {
			L.RaiseError("print exceeds 64 kib")
		}
		if e.setup {
			if len(e.manifest.Diagnostics) >= 64 {
				L.RaiseError("startup diagnostic limit reached")
			}
			e.manifest.Diagnostics = append(e.manifest.Diagnostics, message)
		} else if !e.closing {
			e.emit(L, Effect{Name: "notify", Argument: message})
		}
		return 0
	}))
}
