package plugin

import lua "github.com/yuin/gopher-lua"

// installWindows exposes independent views over shared buffers with staged layout changes.
func (e *engine) installWindows(api *lua.LTable) {
	L := e.L
	methods := L.NewTable()
	methods.RawSetString("open", L.NewFunction(func(L *lua.LState) int {
		bufferID := L.CheckString(1)
		window := SurfaceWindow{Kind: "split", Position: "right"}
		if L.Get(2) != lua.LNil {
			decodeSurface(L, L.CheckTable(2), &window)
		}
		window.BufferID = bufferID
		id := e.reserveSurface("window")
		e.stageSurface(SurfaceMutation{Operation: "window.create", ID: id, Window: &window})
		L.Push(lua.LString(id))
		return 1
	}))
	methods.RawSetString("list", L.NewFunction(func(L *lua.LState) int {
		return pushSurface(L, e.previewSurfaces().Index().Windows)
	}))
	for _, kind := range []string{"get", "configure", "focus", "close"} {
		methods.RawSetString(kind, L.NewFunction(func(L *lua.LState) int {
			id := L.CheckString(1)
			window, err := e.previewSurfaces().Window(id)
			if err != nil {
				L.RaiseError("windows.%s: %v", kind, err)
			}
			if kind == "get" {
				return pushSurface(L, window)
			}
			mutation := SurfaceMutation{Operation: "window." + kind, ID: id, Expected: window.Revision}
			if kind == "configure" {
				decodeSurface(L, L.CheckTable(2), &window)
				mutation.Window = &window
				mutation.Expected = surfaceRevision(L, 3, mutation.Expected)
			} else {
				mutation.Expected = surfaceRevision(L, 2, mutation.Expected)
				if kind == "close" {
					mutation.Operation = "window.delete"
				}
			}
			delta := e.stageSurface(mutation)
			if delta.Window != nil {
				L.Push(lua.LNumber(delta.Window.Revision))
				return 1
			}
			return 0
		}))
	}
	api.RawSetString("windows", methods)
}
