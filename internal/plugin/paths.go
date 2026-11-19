package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/erdajt/wraith/internal/config"
	lua "github.com/yuin/gopher-lua"
)

// installPaths exposes application locations and ordered, explicitly trusted module roots.
func (e *engine) installPaths(api *lua.LTable, directory string, overrides config.Paths) error {
	paths, err := config.Locations()
	if err != nil {
		return fmt.Errorf("lua paths: %w", err)
	}
	for _, location := range []struct {
		value string
		path  *string
	}{{overrides.Data, &paths.Data}, {overrides.Cache, &paths.Cache}} {
		if location.value == "" {
			continue
		}
		absolute, err := filepath.Abs(location.value)
		if err != nil || strings.ContainsRune(absolute, 0) {
			return fmt.Errorf("lua paths: invalid location %q", location.value)
		}
		*location.path = absolute
	}
	state := os.Getenv("XDG_STATE_HOME")
	if !filepath.IsAbs(state) {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("lua state path: %w", err)
		}
		state = filepath.Join(home, ".local", "state")
	}
	locations := map[string]string{"config": directory, "data": paths.Data, "cache": paths.Cache, "state": filepath.Join(state, "wraith")}
	L := e.L
	api.RawSetString("paths", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"stdpath": func(L *lua.LState) int {
			name := L.CheckString(1)
			path, ok := locations[name]
			if !ok {
				L.RaiseError("paths.stdpath: unknown location %s", name)
			}
			L.Push(lua.LString(path))
			return 1
		},
	}))

	roots := []string{directory}
	managed := runtimePatterns(roots)
	pkg := L.GetGlobal("package")
	L.SetField(pkg, "path", lua.LString(strings.Join(managed, ";")))
	methods := map[string]lua.LGFunction{
		"list": func(L *lua.LState) int {
			list := L.NewTable()
			for _, root := range roots {
				list.Append(lua.LString(root))
			}
			L.Push(list)
			return 1
		},
	}
	for _, operation := range []string{"prepend", "append", "remove"} {
		methods[operation] = func(L *lua.LState) int {
			input := L.CheckString(1)
			if input == "" || strings.ContainsAny(input, ";?\x00") {
				L.RaiseError("runtimepath.%s: path is empty or contains semicolon, question mark or nul", operation)
			}
			root, err := filepath.Abs(input)
			if err != nil {
				L.RaiseError("runtimepath.%s: %v", operation, err)
			}
			path, ok := L.GetField(pkg, "path").(lua.LString)
			if !ok {
				L.RaiseError("package.path must be a string")
			}
			unmanaged := slices.DeleteFunc(strings.Split(string(path), ";"), func(pattern string) bool {
				return pattern == "" || slices.Contains(managed, pattern)
			})
			roots = slices.DeleteFunc(roots, func(value string) bool { return value == root })
			switch operation {
			case "prepend":
				roots = append([]string{root}, roots...)
			case "append":
				roots = append(roots, root)
			}
			managed = runtimePatterns(roots)
			patterns := append(slices.Clone(managed), unmanaged...)
			L.SetField(pkg, "path", lua.LString(strings.Join(patterns, ";")))
			return 0
		}
	}
	api.RawSetString("runtimepath", L.SetFuncs(L.NewTable(), methods))
	capabilities := api.RawGetString("capabilities").(*lua.LTable)
	capabilities.RawSetString("runtimepath", lua.LTrue)
	capabilities.RawSetString("stdpath", lua.LTrue)
	return nil
}

// runtimePatterns orders conventional lua directories before legacy root modules.
func runtimePatterns(roots []string) []string {
	patterns := make([]string, 0, len(roots)*4)
	for _, root := range roots {
		patterns = append(patterns, filepath.Join(root, "lua", "?.lua"), filepath.Join(root, "lua", "?", "init.lua"), filepath.Join(root, "?.lua"), filepath.Join(root, "?", "init.lua"))
	}
	return patterns
}
