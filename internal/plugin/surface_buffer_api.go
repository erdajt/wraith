package plugin

import lua "github.com/yuin/gopher-lua"

// installBuffers exposes canonical text with explicit revision checks and paged reads.
func (e *engine) installBuffers(api *lua.LTable) {
	L := e.L
	methods := L.NewTable()
	methods.RawSetString("create", L.NewFunction(func(L *lua.LState) int {
		var buffer SurfaceBuffer
		decodeSurface(L, L.CheckTable(1), &buffer)
		id := e.reserveSurface("buffer")
		e.stageSurface(SurfaceMutation{Operation: "buffer.create", ID: id, Buffer: &buffer})
		L.Push(lua.LString(id))
		return 1
	}))
	methods.RawSetString("list", L.NewFunction(func(L *lua.LState) int {
		return pushSurface(L, e.previewSurfaces().Index().Buffers)
	}))
	methods.RawSetString("native", L.NewFunction(func(L *lua.LState) int {
		slot := L.CheckString(1)
		if slot != "prompt" && slot != "conversation" {
			L.RaiseError("native buffer must be prompt or conversation")
		}
		for _, buffer := range e.previewSurfaces().Index().Buffers {
			if buffer.Projection == slot {
				L.Push(lua.LString(buffer.ID))
				return 1
			}
		}
		L.Push(lua.LNil)
		return 1
	}))
	for _, kind := range []string{"get", "get_lines", "set_lines", "set_spans", "delete"} {
		methods.RawSetString(kind, L.NewFunction(func(L *lua.LState) int {
			id := L.CheckString(1)
			buffer, err := e.previewSurfaces().Buffer(id)
			if err != nil {
				L.RaiseError("buffers.%s: %v", kind, err)
			}
			if kind == "get" {
				return pushSurface(L, buffer)
			}
			if kind == "get_lines" {
				start, end := L.OptInt(2, 0), L.OptInt(3, -1)
				page, err := e.previewSurfaces().BufferLines(id, start, end)
				if err != nil {
					L.RaiseError("buffers.get_lines: %v", err)
				}
				return pushSurface(L, page)
			}
			mutation := SurfaceMutation{Operation: "buffer." + kind, ID: id, Expected: buffer.Revision}
			switch kind {
			case "set_lines":
				mutation.Start, mutation.End = L.CheckInt(2), L.CheckInt(3)
				if mutation.End == -1 {
					mutation.End = len(buffer.Lines)
				}
				decodeSurface(L, L.CheckTable(4), &mutation.Lines)
				mutation.Expected = surfaceRevision(L, 5, buffer.Revision)
			case "set_spans":
				decodeSurface(L, L.CheckTable(2), &mutation.Spans)
				mutation.Expected = surfaceRevision(L, 3, buffer.Revision)
			case "delete":
				mutation.Expected = surfaceRevision(L, 2, buffer.Revision)
			}
			delta := e.stageSurface(mutation)
			if delta.Buffer != nil {
				L.Push(lua.LNumber(delta.Buffer.Revision))
				return 1
			}
			return 0
		}))
	}
	api.RawSetString("buffers", methods)
}
