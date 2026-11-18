package plugin

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	lua "github.com/yuin/gopher-lua"
)

type cleanupHook struct {
	id           uint64
	owner        string
	function     *lua.LFunction
	dependencies []string
}

// addCleanup gives each hook a stable identity so rollback disposes only newly owned resources.
func (e *engine) addCleanup(function *lua.LFunction) {
	e.serial++
	hook := cleanupHook{id: e.serial, owner: e.owner, function: function}
	hook.dependencies = e.cleanupDependencies(e.owner)
	e.cleanup = append(e.cleanup, hook)
}

// disposeHooks releases resources on the VM owner without retaining a cancelled callback deadline.
func (e *engine) disposeHooks(hooks []cleanupHook) error {
	previous, closing, owner := e.L.Context(), e.closing, e.owner
	e.closing = true
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	e.L.SetContext(ctx)
	defer func() { e.L.SetContext(previous); e.closing = closing; e.owner = owner }()
	hooks = orderCleanupHooks(hooks)
	var failures []error
	for i := len(hooks) - 1; i >= 0; i-- {
		if err := ctx.Err(); err != nil {
			failures = append(failures, fmt.Errorf("plugin cleanup %s: %w", hooks[i].owner, err))
			break
		}
		e.owner = hooks[i].owner
		if err := e.L.CallByParam(lua.P{Fn: hooks[i].function, NRet: 0, Protect: true}); err != nil {
			failures = append(failures, fmt.Errorf("plugin cleanup %s: %w", hooks[i].owner, err))
		}
	}
	return errors.Join(failures...)
}

// teardown releases committed and pending resources in reverse registration order.
func (e *engine) teardown(start int) error {
	e.closing = true
	hooks := append(slices.Clone(e.cleanup[start:]), e.disposing...)
	e.cleanup, e.disposing = e.cleanup[:start], nil
	return e.disposeHooks(hooks)
}

// Notices reports asynchronous cleanup failures without making the ui wait for teardown.
func (r *Runtime) Notices() <-chan string { return r.notices }

// reportCleanup bounds diagnostic storage while retaining the most recent cleanup failures.
func (r *Runtime) reportCleanup(err error) {
	if err == nil {
		return
	}
	message := err.Error()
	if len(message) > 4096 {
		message = message[:4096] + "…"
	}
	r.mu.Lock()
	r.diagnostics = append(r.diagnostics, message)
	if len(r.diagnostics) > 64 {
		r.diagnostics = slices.Clone(r.diagnostics[len(r.diagnostics)-64:])
	}
	r.mu.Unlock()
	select {
	case r.notices <- message:
	default:
		select {
		case <-r.notices:
		default:
		}
		r.notices <- message
	}
}

// Diagnostics returns retained startup and cleanup diagnostics independently of notice delivery.
func (r *Runtime) Diagnostics() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.diagnostics)
}
