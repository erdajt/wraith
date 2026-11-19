package plugin

import (
	"context"
	"math"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// installTimers stages one-shot or repeating timers under the current package owner.
func (e *engine) installTimers(api *lua.LTable) {
	api.RawSetString("timer", e.L.SetFuncs(e.L.NewTable(), map[string]lua.LGFunction{
		"start": func(L *lua.LState) int {
			delay := L.CheckNumber(1)
			callback := L.CheckFunction(2)
			repeat := lua.LNumber(0)
			if options := L.OptTable(3, nil); options != nil {
				options.ForEach(func(key, value lua.LValue) {
					var ok bool
					if key.String() == "repeat_ms" {
						repeat, ok = value.(lua.LNumber)
					}
					if !ok || repeat < 1 {
						L.RaiseError("timer.start: options require positive repeat_ms")
					}
				})
			}
			if delay < 1 || delay > 86400000 || math.Trunc(float64(delay)) != float64(delay) || repeat > 86400000 || math.Trunc(float64(repeat)) != float64(repeat) {
				L.RaiseError("timer.start: delays require integer milliseconds between 1 and 86400000")
			}
			initial, interval := time.Duration(delay)*time.Millisecond, time.Duration(repeat)*time.Millisecond
			id := e.reserveResource(L, "timer", callback, func(ctx context.Context, sink *resourceSink) resourceResult {
				started := time.Now()
				timer := time.NewTimer(initial)
				defer timer.Stop()
				var tick int
				for {
					select {
					case <-ctx.Done():
						return resourceResult{Data: map[string]any{"tick": tick, "elapsed_ms": time.Since(started).Milliseconds()}, Err: ctx.Err()}
					case <-timer.C:
						tick++
						data := map[string]any{"tick": tick, "elapsed_ms": time.Since(started).Milliseconds()}
						if interval == 0 {
							return resourceResult{Data: data}
						}
						if err := sink.Emit("tick", data, false); err != nil {
							return resourceResult{Data: data, Err: err}
						}
						timer.Reset(interval)
					}
				}
			})
			L.Push(lua.LString(id))
			return 1
		},
	}))
}
