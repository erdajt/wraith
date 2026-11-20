package plugin

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/erdajt/wraith/internal/config"
	lua "github.com/yuin/gopher-lua"
	"golang.org/x/sys/unix"
)

type runtimeHosts struct {
	services    *resourceManager
	surfaces    *SurfaceStore
	invocations *invocationManager
	paths       config.Paths
	packages    []PackageRoot
}

// load initializes trusted user code without sourcing project files or environment module paths.
func load(parent context.Context, directory string, c config.Config, actions []string, hosts ...runtimeHosts) (out *engine, err error) {
	directory, err = filepath.Abs(directory)
	if err != nil {
		return nil, err
	}
	L := lua.NewState(lua.Options{SkipOpenLibs: true})
	e := &engine{L: L, manifest: Manifest{Config: c}, functions: map[string]*lua.LFunction{}, events: map[string][]string{}, actions: map[string]bool{}, builtins: map[string]bool{}, setup: true}
	if len(hosts) > 0 {
		e.services, e.surfaces, e.invocations = hosts[0].services, hosts[0].surfaces, hosts[0].invocations
	} else {
		e.services = newResourceManager(parent)
		e.surfaces = newSurfaceStore(e.services.prefix)
		e.invocations = newInvocationManager(e.services.prefix, e.services.discard)
	}
	e.surfaceOwners = map[string]string{"config": e.surfaces.OwnerLease("config"), "core": e.surfaces.OwnerLease("core")}
	e.initOwnership()
	for _, name := range actions {
		e.actions[name] = true
		e.builtins[name] = true
	}
	ok := false
	defer func() {
		if !ok {
			e.invocations.close()
			e.services.close()
			e.surfaces.close()
			err = errors.Join(err, e.teardown(0))
			L.Close()
		}
	}()
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	L.SetContext(ctx)
	for _, open := range []lua.LGFunction{lua.OpenPackage, lua.OpenBase, lua.OpenTable, lua.OpenString, lua.OpenMath, lua.OpenCoroutine, lua.OpenIo, lua.OpenOs, lua.OpenDebug} {
		L.Push(L.NewFunction(open))
		L.Call(0, 0)
	}
	e.nativeModules, err = captureNativeModules(L)
	if err != nil {
		return nil, err
	}
	if err := e.installLoader(directory); err != nil {
		return nil, err
	}
	api := L.NewTable()
	api.RawSetString("api_version", lua.LNumber(2))
	L.SetGlobal("wraith", api)
	e.installConfig(api)
	e.installJSON(api)
	e.installFilesystem(api)
	e.installTimers(api)
	e.installHTTP(api)
	e.installRegistration(api)
	e.installEventTargets(api)
	e.installReadyReplay(api)
	e.installHooks(api)
	e.installExtensions(api)
	e.installInvocations(api)
	e.installWorkspace(api)
	e.installWorkspaceQueries(api)
	e.installPlugins(api)
	e.installSchedule(api)
	e.installDialogs(api)
	e.installBuffers(api)
	e.installBufferMappings(api)
	e.installWindows(api)
	e.installRegions(api)
	e.installResources(api)
	e.installProcess(api)
	e.installCoroutines()
	e.installDiagnostics(api)
	var paths config.Paths
	if len(hosts) > 0 {
		paths = hosts[0].paths
	}
	if err := e.installPaths(api, directory, paths); err != nil {
		return nil, err
	}

	if len(hosts) > 0 {
		if err := e.installPackageModules(hosts[0].packages); err != nil {
			return nil, err
		}
	}

	e.installModuleDiscovery(api)
	e.installModuleProviders(api)
	e.guardHookAPIs(api)
	fn, err := loadFile(L, filepath.Join(directory, "init.lua"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err == nil {
		if err := L.CallByParam(lua.P{Fn: fn, NRet: 1, Protect: true}); err != nil {
			return nil, err
		}
		if cleanup, ok := L.Get(-1).(*lua.LFunction); ok {
			e.addCleanup(cleanup)
		}
		L.Pop(1)
	}
	if err := e.validateRegistry(); err != nil {
		return nil, err
	}
	for event, ids := range e.events {
		if len(ids) > 0 {
			e.manifest.Events = append(e.manifest.Events, event)
		}
	}
	slices.Sort(e.manifest.Events)
	e.services.discard(e.released)
	e.released = nil
	for _, owner := range e.surfaceReleased {
		e.surfaces.RevokeOwner(owner)
	}
	e.surfaceReleased = nil
	hooks := e.disposing
	e.disposing = nil
	if err := e.disposeRegistry(registryDisposal{hooks: hooks, modules: e.retiredModuleExecutions()}); err != nil {
		return nil, err
	}
	e.acceptExtensions()
	e.setup, ok = false, true
	return e, nil
}

// loadFile bounds source reads before compiling a text-only chunk with its path in errors.
func loadFile(L *lua.LState, path string) (*lua.LFunction, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|unix.O_NONBLOCK, 0)
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
