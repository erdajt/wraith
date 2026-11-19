package plugin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	lua "github.com/yuin/gopher-lua"
	"golang.org/x/sys/unix"
)

type moduleSource struct {
	name, path, root string
	preload          *lua.LFunction
}

// installModuleDiscovery exposes selected sources without allowing metadata to redirect their loaders.
func (e *engine) installModuleDiscovery(api *lua.LTable) {
	L := e.L
	api.RawSetString("modules", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"discover": func(L *lua.LState) int {
			ctx := L.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			sources, err := e.discoverModules(ctx, L.CheckString(1))
			if err != nil {
				L.RaiseError("modules.discover: %v", err)
			}
			result := L.NewTable()
			for _, source := range sources {
				record := L.NewTable()
				record.RawSetString("name", lua.LString(source.name))
				kind, identity := "file", "file:"+source.path
				if source.preload != nil {
					kind, identity = "preload", fmt.Sprintf("preload:%s:%p", source.name, source.preload)
				} else {
					record.RawSetString("path", lua.LString(source.path))
				}
				record.RawSetString("kind", lua.LString(kind))
				record.RawSetString("identity", lua.LString(identity))
				record.RawSetString("load", L.NewFunction(func(L *lua.LState) int {
					if e.pureHook {
						e.hookViolation = errors.New("modules.load is unavailable during pure hooks")
						L.RaiseError("%v", e.hookViolation)
					}
					if ctx := L.Context(); ctx != nil && ctx.Err() != nil {
						L.RaiseError("module %s: %v", source.name, ctx.Err())
					}
					chunk := source.preload
					if chunk == nil {
						path, err := modulePath(source.root, source.path)
						if err != nil {
							L.RaiseError("module %s: %v", source.name, err)
						}
						chunk, err = loadFile(L, path)
						if err != nil {
							L.RaiseError("module %s: %v", source.name, err)
						}
					}
					if ctx := L.Context(); ctx != nil && ctx.Err() != nil {
						L.RaiseError("module %s: %v", source.name, ctx.Err())
					}
					L.SetTop(0)
					if err := L.CallByParam(lua.P{Fn: chunk, NRet: lua.MultRet, Protect: true}, lua.LString(source.name)); err != nil {
						L.RaiseError("module %s: %v", source.name, err)
					}
					return L.GetTop()
				}))
				result.Append(record)
			}
			L.Push(result)
			return 1
		},
	}))
	api.RawGetString("capabilities").(*lua.LTable).RawSetString("modules", lua.LTrue)
}

// selectedNamespace distinguishes a missing bound source from an ordinary runtime module.
func (e *engine) selectedNamespace(name string) bool {
	for _, namespace := range e.moduleNamespaces {
		if name == namespace || strings.HasPrefix(name, namespace+".") {
			return true
		}
	}
	return false
}

// discoverModules snapshots immediate sources from one selected namespace or explicit runtime base.
func (e *engine) discoverModules(ctx context.Context, name string) ([]moduleSource, error) {
	if len(name) > 128 || !namespaceName.MatchString(name) {
		return nil, errors.New("invalid module name")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	pkg := e.L.GetGlobal("package")
	preload, ok := e.L.GetField(pkg, "preload").(*lua.LTable)
	if !ok {
		return nil, errors.New("package.preload must be a table")
	}
	if fn, ok := preload.RawGetString(name).(*lua.LFunction); ok && fn != e.moduleSources[name].preload {
		return []moduleSource{{name: name, preload: fn}}, nil
	}

	var sources []moduleSource
	if e.selectedNamespace(name) {
		for _, source := range e.moduleSources {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			base := filepath.Join(source.root, "lua", filepath.FromSlash(strings.ReplaceAll(name, ".", "/")))
			parent := filepath.Dir(source.path)
			if source.name == name || parent == base || filepath.Base(source.path) == "init.lua" && filepath.Dir(parent) == base {
				sources = append(sources, source)
			}
		}
		if len(sources) == 0 {
			return nil, fmt.Errorf("module %s is absent from its selected package", name)
		}
	} else {
		if len(e.runtimeRoots) > 128 {
			return nil, errors.New("module discovery exceeds 128 runtime roots")
		}
	runtimeRoots:
		for _, root := range e.runtimeRoots {
			for _, prefix := range []string{"lua", ""} {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				var found bool
				var err error
				sources, found, err = discoverModuleFiles(ctx, root, filepath.Join(prefix, filepath.FromSlash(strings.ReplaceAll(name, ".", "/"))), name)
				if err != nil {
					return nil, err
				}
				if found {
					break runtimeRoots
				}
			}
		}
	}
	if len(sources) > 256 {
		return nil, errors.New("module discovery exceeds 256 sources")
	}
	slices.SortFunc(sources, func(a, b moduleSource) int { return strings.Compare(a.name, b.name) })
	for i, source := range sources {
		if i > 0 && sources[i-1].name == source.name {
			return nil, fmt.Errorf("ambiguous module %s", source.name)
		}
		if !namespaceName.MatchString(source.name) || len(source.name) > 128 {
			return nil, fmt.Errorf("invalid discovered module %q", source.name)
		}
		if fn, ok := preload.RawGetString(source.name).(*lua.LFunction); ok && fn != e.moduleSources[source.name].preload {
			sources[i] = moduleSource{name: source.name, preload: fn}
			continue
		}
		if e.selectedNamespace(source.name) {
			bound, ok := e.moduleSources[source.name]
			if !ok {
				return nil, fmt.Errorf("module %s is absent from its selected package", source.name)
			}
			source = bound
		}
		path, err := modulePath(source.root, source.path)
		if err != nil {
			return nil, err
		}
		root, err := filepath.EvalSymlinks(source.root)
		if err != nil {
			return nil, err
		}
		sources[i] = moduleSource{name: source.name, path: path, root: root}
	}
	return sources, ctx.Err()
}

// modulePath follows an explicit root while rejecting symlinks in discovered child components.
func modulePath(root, path string) (string, error) {
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("module source is outside its root")
	}
	current, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("module child path is a symlink: %s", current)
		}
	}
	return current, nil
}

// discoverModuleFiles enumerates one immediate directory without truncating or merging later roots.
func discoverModuleFiles(ctx context.Context, root, relative, name string) ([]moduleSource, bool, error) {
	base := filepath.Join(root, relative)
	var sources []moduleSource
	_, err := modulePath(root, base+".lua")
	if err == nil {
		sources = append(sources, moduleSource{name: name, root: root, path: filepath.Join(root, relative+".lua")})
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, false, err
	}
	directory, err := modulePath(root, base)
	if errors.Is(err, os.ErrNotExist) {
		return sources, len(sources) > 0, nil
	}
	if err != nil {
		return nil, false, err
	}
	file, err := os.OpenFile(directory, os.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, false, err
	}
	defer file.Close()
	entries, err := file.ReadDir(1025)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, false, err
	}
	if len(entries) > 1024 {
		return nil, false, fmt.Errorf("module directory exceeds 1024 entries: %s", directory)
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil, false, fmt.Errorf("module child path is a symlink: %s", filepath.Join(directory, entry.Name()))
		}
		child, module := filepath.Join(base, entry.Name()), name+"."+strings.TrimSuffix(entry.Name(), ".lua")
		if entry.IsDir() {
			module = name + "." + entry.Name()
			child = filepath.Join(child, "init.lua")
			if _, err := modulePath(root, child); errors.Is(err, os.ErrNotExist) {
				continue
			} else if err != nil {
				return nil, false, err
			}
		} else if !strings.HasSuffix(entry.Name(), ".lua") {
			continue
		} else if entry.Name() == "init.lua" {
			module = name
		}
		sources = append(sources, moduleSource{name: module, path: child, root: root})
	}
	return sources, true, nil
}
