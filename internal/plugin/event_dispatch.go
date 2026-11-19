package plugin

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// invokeObservers isolates each subscriber while preserving successful earlier changes.
func (e *engine) invokeObservers(parent context.Context, event string) error {
	ids := slices.Clone(e.events[event])
	for _, id := range ids {
		if err := parent.Err(); err != nil {
			return err
		}
		if !slices.Contains(e.events[event], id) {
			continue
		}
		owner := e.callbackOwners[id]
		fn := e.functions[id]
		if fn == nil {
			e.observerErrors = append(e.observerErrors, fmt.Errorf("event %s observer %s: callback no longer exists", event, owner))
			continue
		}

		before := e.captureRegistry()
		e.owner = owner
		ctx, cancel := context.WithTimeout(parent, 50*time.Millisecond)
		e.L.SetContext(ctx)
		argument, err := e.invocationContext(encodeValue(e.L, e.snapshot), nil)
		if err == nil {
			err = e.L.CallByParam(lua.P{Fn: fn, NRet: 0, Protect: true}, argument)
		}
		if ctx.Err() != nil {
			err = errors.Join(err, ctx.Err())
		}
		cancel()
		e.L.SetContext(parent)
		if parent.Err() != nil {
			return parent.Err()
		}
		if err == nil {
			err = e.validateRegistry()
		}
		if err != nil {
			hooks := e.restoreRegistry(before)
			err = errors.Join(err, e.disposeHooks(hooks))
			e.observerErrors = append(e.observerErrors, fmt.Errorf("event %s observer %s: %w", event, owner, err))
		}
	}
	return parent.Err()
}
