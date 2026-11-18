package plugin

import lua "github.com/yuin/gopher-lua"

// installCoroutines rebinds suspended threads to the current callback's cancellation scope.
func (e *engine) installCoroutines() {
	L := e.L
	module := L.GetGlobal("coroutine").(*lua.LTable)
	create := module.RawGetString("create").(*lua.LFunction).GFunction
	resume := module.RawGetString("resume").(*lua.LFunction).GFunction
	module.RawSetString("resume", L.NewFunction(func(L *lua.LState) int {
		L.CheckThread(1).SetContext(L.Context())
		return resume(L)
	}))
	module.RawSetString("wrap", L.NewFunction(func(L *lua.LState) int {
		create(L)
		thread := L.CheckThread(L.GetTop())
		L.Pop(1)
		L.Push(L.NewFunction(func(L *lua.LState) int {
			thread.SetContext(L.Context())
			L.Insert(thread, 1)
			n := resume(L)
			first := L.GetTop() - n + 1
			if !L.ToBool(first) {
				L.RaiseError("%s", L.Get(first+1))
			}
			L.Remove(first)
			return n - 1
		}))
		return 1
	}))
}
