package plugin

import (
	"errors"
	"fmt"

	lua "github.com/yuin/gopher-lua"
)

// callPackage invokes a captured export with its owner and an independent registration savepoint.
func (e *engine) callPackage(name, method string, args []lua.LValue) (value lua.LValue, err error) {
	record, ok := e.packages[name]
	if !ok || record.loading || record.module == nil {
		return lua.LNil, fmt.Errorf("plugin is not fully loaded: %s", name)
	}
	if method == "setup" || method == "teardown" {
		return lua.LNil, errors.New("setup and teardown are lifecycle methods")
	}
	fn, ok := record.module.RawGetString(method).(*lua.LFunction)
	if !ok {
		return lua.LNil, fmt.Errorf("plugin %s has no method %s", name, method)
	}
	if len(e.packageCalls) >= 32 {
		return lua.LNil, errors.New("plugin method nesting exceeds 16 calls")
	}

	before := e.captureRegistry()
	owner, depth, top := e.owner, len(e.packageCalls), e.L.GetTop()
	e.packageCalls = append(e.packageCalls, owner, record.owner)
	e.owner = record.owner
	defer func() {
		e.L.SetTop(top)
		e.packageCalls = e.packageCalls[:depth]
		e.owner = owner
	}()
	defer func() {
		if err != nil {
			disposal := e.restoreRegistry(before)
			err = errors.Join(err, e.disposeRegistry(disposal))
		}
	}()
	if err = e.L.CallByParam(lua.P{Fn: fn, NRet: 1, Protect: true}, args...); err != nil {
		return lua.LNil, fmt.Errorf("plugin %s method %s: %w", name, method, err)
	}
	if err = e.validateRegistry(); err != nil {
		return lua.LNil, err
	}
	return e.L.Get(-1), nil
}
