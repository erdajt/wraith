package plugin

import (
	"slices"
	"strings"

	"github.com/erdajt/wraith/internal/keymap"
	lua "github.com/yuin/gopher-lua"
)

// installBufferMappings gives each buffer owned overrides above the workspace keymap.
func (e *engine) installBufferMappings(api *lua.LTable) {
	L := e.L
	methods := api.RawGetString("buffers").(*lua.LTable)
	for _, kind := range []string{"set_keymap", "del_keymap"} {
		methods.RawSetString(kind, L.NewFunction(func(L *lua.LState) int {
			id := L.CheckString(1)
			buffer, err := e.previewSurfaces().Buffer(id)
			if err != nil {
				L.RaiseError("buffers.%s: %v", kind, err)
			}
			binding := keymap.Binding{Buffer: id, Mode: L.CheckString(2), Keys: L.CheckString(3)}
			mutation := SurfaceMutation{Operation: "buffer." + kind, ID: id, Expected: buffer.Revision, Binding: &binding}
			if kind == "del_keymap" {
				mutation.Expected = surfaceRevision(L, 4, buffer.Revision)
				e.stageSurface(mutation)
				return 0
			}
			binding.Description = L.OptString(5, "lua buffer mapping")
			var callback *lua.LFunction
			switch target := L.Get(4).(type) {
			case lua.LString:
				binding.Action = string(target)
			case *lua.LFunction:
				callback, binding.Action = target, "lua.callback.pending"
			case *lua.LTable:
				var native struct {
					Keys string `json:"keys"`
				}
				decodeSurface(L, target, &native)
				binding.Input = native.Keys
			default:
				L.RaiseError("buffer mapping requires an action, native keys or callback")
			}
			if _, err := keymap.New([]keymap.Binding{binding}); err != nil {
				L.RaiseError("buffers.set_keymap: %v", err)
			}
			mutation.Expected = surfaceRevision(L, 6, buffer.Revision)
			if callback == nil && binding.Input == "" && !e.actions[binding.Action] && e.functions[binding.Action] == nil {
				L.RaiseError("unknown buffer mapping action: %s", binding.Action)
			}
			callbackID := ""
			accepted := false
			defer func() {
				if !accepted && callbackID != "" {
					delete(e.functions, callbackID)
					delete(e.callbackOwners, callbackID)
					e.manifest.Callbacks = slices.DeleteFunc(e.manifest.Callbacks, func(id string) bool { return id == callbackID })
				}
			}()
			if callback != nil {
				callbackID = e.callback(L, callback)
				binding.Action = callbackID
			}
			mutation.TargetOwner = e.callbackOwners[binding.Action]
			if name, ok := strings.CutPrefix(binding.Action, "command."); ok {
				if layers := e.commandLayers[name]; len(layers) > 0 {
					mutation.TargetOwner = layers[len(layers)-1].owner
				}
			}
			delta := e.stageSurface(mutation)
			accepted = true
			if callback != nil {
				e.dirty = !e.setup
			}
			L.Push(lua.LNumber(delta.Buffer.Revision))
			return 1
		}))
	}
}
