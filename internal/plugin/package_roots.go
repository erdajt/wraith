package plugin

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/erdajt/wraith/internal/config"
	lua "github.com/yuin/gopher-lua"
)

type PackageRoot struct {
	Directory  string
	Namespaces []string
}

type Options struct {
	Paths    config.Paths
	Packages []PackageRoot
}

var namespaceName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_-]*(\.[a-zA-Z_][a-zA-Z0-9_-]*)*$`)

// installPackageModules binds selected namespaces to explicit source roots before user initialization.
func (e *engine) installPackageModules(roots []PackageRoot) error {
	if len(roots) > 32 {
		return errors.New("package roots exceed 32")
	}
	namespaces := []string{}
	modules := map[string]moduleSource{}
	visited := 0
	for _, root := range roots {
		if !filepath.IsAbs(root.Directory) || strings.ContainsAny(root.Directory, ";?\x00") || len(root.Namespaces) == 0 || len(root.Namespaces) > 32 {
			return errors.New("package roots require an absolute directory and between 1 and 32 namespaces")
		}
		for _, namespace := range root.Namespaces {
			if !namespaceName.MatchString(namespace) || len(namespace) > 128 {
				return fmt.Errorf("invalid package namespace %q", namespace)
			}
			for _, previous := range namespaces {
				if namespace == previous || strings.HasPrefix(namespace, previous+".") || strings.HasPrefix(previous, namespace+".") {
					return fmt.Errorf("overlapping package namespaces: %s and %s", namespace, previous)
				}
			}
			namespaces = append(namespaces, namespace)
		}
		base := filepath.Join(root.Directory, "lua")
		count := len(modules)
		err := filepath.WalkDir(base, func(path string, item fs.DirEntry, err error) error {
			if ctx := e.L.Context(); ctx != nil && ctx.Err() != nil {
				return ctx.Err()
			}
			visited++
			if visited > 8192 {
				return errors.New("package discovery exceeds 8192 entries")
			}
			if err != nil {
				return err
			}
			if item.IsDir() {
				return nil
			}
			if item.Type()&os.ModeSymlink != 0 {
				return errors.New("package module paths must not contain symlinks")
			}
			if !strings.HasSuffix(path, ".lua") {
				return nil
			}
			relative, err := filepath.Rel(base, path)
			if err != nil {
				return err
			}
			name := strings.TrimSuffix(filepath.ToSlash(relative), ".lua")
			name = strings.ReplaceAll(strings.TrimSuffix(name, "/init"), "/", ".")
			selected := false
			for _, namespace := range root.Namespaces {
				selected = selected || name == namespace || strings.HasPrefix(name, namespace+".")
			}
			if !selected {
				return nil
			}
			if modules[name].path != "" {
				return fmt.Errorf("ambiguous package module %s", name)
			}
			modules[name] = moduleSource{name: name, path: path, root: root.Directory}
			if len(modules) > 4096 {
				return errors.New("package modules exceed 4096")
			}
			return nil
		})
		if err != nil {
			return err
		}
		if len(modules) == count {
			return errors.New("package root contains no modules in its declared namespaces")
		}
	}

	pkg := e.L.GetGlobal("package")
	loaded := e.L.GetField(pkg, "loaded").(*lua.LTable)
	for _, namespace := range namespaces {
		var conflict string
		loaded.ForEach(func(key, value lua.LValue) {
			name := key.String()
			if value != lua.LNil && (name == namespace || strings.HasPrefix(name, namespace+".")) {
				conflict = name
			}
		})
		if conflict != "" {
			return fmt.Errorf("package namespace contains an already loaded module: %s", conflict)
		}
	}
	loaders := e.L.GetField(pkg, "loaders").(*lua.LTable)
	for index := loaders.Len(); index >= 2; index-- {
		loaders.RawSetInt(index+1, loaders.RawGetInt(index))
	}
	loaders.RawSetInt(2, e.L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		for _, namespace := range namespaces {
			if name == namespace || strings.HasPrefix(name, namespace+".") {
				L.RaiseError("module %s is absent from its selected package", name)
			}
		}
		return 0
	}))
	preload := e.L.GetField(pkg, "preload").(*lua.LTable)
	for name, source := range modules {
		source.preload = e.L.NewFunction(func(L *lua.LState) int {
			chunk, err := loadFile(L, source.path)
			if err != nil {
				L.RaiseError("module %s: %v", name, err)
			}
			if err := L.CallByParam(lua.P{Fn: chunk, NRet: 1, Protect: true}, L.Get(1)); err != nil {
				L.RaiseError("module %s: %v", name, err)
			}
			return 1
		})
		modules[name] = source
		preload.RawSetString(name, source.preload)
	}
	e.moduleSources, e.moduleNamespaces = modules, namespaces
	return nil
}
