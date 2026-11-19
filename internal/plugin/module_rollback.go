package plugin

import (
	"errors"

	lua "github.com/yuin/gopher-lua"
)

// rollbackModule retains failed ownership evidence across source and cleanup disposal.
func (e *engine) rollbackModule(L *lua.LState, name string, native nativeModules, initial lua.LValue, before registryState, cause error) error {
	changed := native.validate(L)
	if changed != nil {
		cause = errors.Join(cause, changed, e.poisonModule(name, changed.Error()))
	}
	cause = errors.Join(cause, e.disposeRegistry(e.restoreRegistry(before)))
	if after := native.validate(L); after != nil && changed == nil {
		cause = errors.Join(cause, after, e.poisonModule(name, after.Error()))
	}
	if value := native.loaded.RawGetString(name); value != lua.LNil && value != initial {
		failure := errors.New("failed managed module request left an unverified cache value; use /reload")
		cause = errors.Join(cause, failure, e.poisonModule(name, failure.Error()))
	}
	return cause
}
