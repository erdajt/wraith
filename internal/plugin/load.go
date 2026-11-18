package plugin

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/erdajt/wraith/internal/config"
	lua "github.com/yuin/gopher-lua"
	"golang.org/x/sys/unix"
)

var moduleName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*(\.[a-zA-Z_][a-zA-Z0-9_]*)*$`)

// load confines module lookup to the user configuration root and closes failed candidates.
func load(parent context.Context, directory string, c config.Config, actions []string) (*engine, func(), error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, nil, err
	}
	L := lua.NewState(lua.Options{SkipOpenLibs: true})
	e := &engine{L: L, manifest: Manifest{Config: c}, functions: map[string]*lua.LFunction{}, events: map[string][]string{}, actions: map[string]bool{}, setup: true}
	for _, name := range actions {
		e.actions[name] = true
	}
	ok := false
	defer func() {
		if !ok {
			L.Close()
			_ = root.Close()
		}
	}()
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	L.SetContext(ctx)
	for _, open := range []lua.LGFunction{lua.OpenPackage, lua.OpenBase, lua.OpenTable, lua.OpenString, lua.OpenMath, lua.OpenCoroutine} {
		L.Push(L.NewFunction(open))
		L.Call(0, 0)
	}
	for _, name := range []string{"dofile", "loadfile", "print", "_printregs", "collectgarbage"} {
		L.SetGlobal(name, lua.LNil)
	}
	packageTable := L.GetGlobal("package")
	L.SetField(packageTable, "loadlib", lua.LNil)
	L.SetField(packageTable, "path", lua.LString("<config>/?.lua;<config>/?/init.lua"))
	loaders := L.NewTable()
	loaders.Append(L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		if !moduleName.MatchString(name) || len(name) > 256 {
			L.RaiseError("invalid local module name: %s", name)
		}
		path := strings.ReplaceAll(name, ".", "/")
		fn, err := loadFile(L, root, path+".lua")
		if errors.Is(err, os.ErrNotExist) {
			fn, err = loadFile(L, root, path+"/init.lua")
		}
		if err != nil {
			L.RaiseError("module %s: %v", name, err)
		}
		L.Push(fn)
		return 1
	}))
	L.SetField(packageTable, "loaders", loaders)
	L.SetField(L.Get(lua.RegistryIndex), "_LOADERS", loaders)
	api := L.NewTable()
	api.RawSetString("api_version", lua.LNumber(1))
	L.SetGlobal("wraith", api)
	e.installConfig(api)
	e.installRegistration(api)
	e.installWorkspace(api)
	e.installPlugins(api)
	e.installCoroutines()

	fn, err := loadFile(L, root, "init.lua")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, nil, err
	}
	if err == nil {
		if err := L.CallByParam(lua.P{Fn: fn, NRet: 1, Protect: true}); err != nil {
			return nil, nil, err
		}
		if cleanup, ok := L.Get(-1).(*lua.LFunction); ok {
			e.cleanup = append(e.cleanup, cleanup)
		}
		L.Pop(1)
	}
	for _, binding := range e.manifest.Config.Keymaps {
		if !e.actions[binding.Action] && e.functions[binding.Action] == nil {
			return nil, nil, fmt.Errorf("keymap %q: unknown action %s", binding.Keys, binding.Action)
		}
	}
	if err := e.manifest.Config.Validate(); err != nil {
		return nil, nil, err
	}
	for event, ids := range e.events {
		if len(ids) > 0 {
			e.manifest.Events = append(e.manifest.Events, event)
		}
	}
	slices.Sort(e.manifest.Events)
	e.setup, ok = false, true
	return e, func() { _ = root.Close() }, nil
}

// loadFile bounds source reads before compiling a text-only chunk with its path in errors.
func loadFile(L *lua.LState, root *os.Root, path string) (*lua.LFunction, error) {
	f, err := root.OpenFile(path, os.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("lua source must be a regular file: %s", path)
	}
	data, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err != nil || len(data) > 1<<20 {
		return nil, fmt.Errorf("cannot read lua source within 1 mib: %s", path)
	}
	return L.Load(bytes.NewReader(data), "@"+path)
}
