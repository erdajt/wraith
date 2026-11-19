package plugin

import (
	"encoding/json"

	lua "github.com/yuin/gopher-lua"
)

// installInvocations stages stream replies without making Lua wait for managed callbacks.
func (e *engine) installInvocations(api *lua.LTable) {
	L := e.L
	table := L.NewTable()
	for _, kind := range []string{"emit", "finish", "fail"} {
		table.RawSetString(kind, L.NewFunction(func(L *lua.LState) int {
			id := L.CheckString(1)
			if id != e.invocationID || !e.invocations.valid(id) {
				L.RaiseError("invocation is canceled or does not belong to this callback")
			}
			effect := InvocationEffect{ID: id, Kind: kind}
			if kind == "fail" {
				effect.Error = L.CheckString(2)
				if effect.Error == "" || len(effect.Error) > 4096 {
					L.RaiseError("invocation failure requires a message within 4096 bytes")
				}
			} else {
				value, err := e.jsonCodec.toGo(L.Get(2), 0, &jsonBudget{active: map[*lua.LTable]bool{}})
				if err != nil {
					L.RaiseError("invocation.%s: %v", kind, err)
				}
				data, err := json.Marshal(value)
				if err != nil {
					L.RaiseError("invocation.%s: %v", kind, err)
				}
				effect.Data, err = invocationPayload(data)
				if err != nil {
					L.RaiseError("invocation.%s: %v", kind, err)
				}
			}
			e.emit(L, Effect{Name: "invocation.reply", Argument: id, Data: effect})
			return 0
		}))
	}
	table.RawSetString("cancelled", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LBool(!e.invocations.valid(L.CheckString(1))))
		return 1
	}))
	api.RawSetString("invocation", table)
}

// invocationContext preserves explicit JSON containers while adding immutable call identity.
func (e *engine) invocationContext(value lua.LValue, start *InvocationStart) (lua.LValue, error) {
	if e.invocationID == "" {
		return value, nil
	}
	table, ok := value.(*lua.LTable)
	if !ok {
		table = e.L.NewTable()
	}
	metadata := e.L.NewTable()
	metadata.RawSetString("id", lua.LString(e.invocationID))
	if start != nil {
		metadata.RawSetString("kind", lua.LString(start.Binding.Kind))
		metadata.RawSetString("name", lua.LString(start.Binding.Name))
		metadata.RawSetString("method", lua.LString(start.Binding.Method))
		var decoded any
		if err := json.Unmarshal(start.Data, &decoded); err != nil {
			return nil, err
		}
		data, err := e.jsonCodec.toLua(decoded, 0, &jsonBudget{})
		if err != nil {
			return nil, err
		}
		table.RawSetString("data", data)
	}
	table.RawSetString("invocation", metadata)
	return table, nil
}

// acceptExtensions keeps accepted callbacks alive while a replacement awaits host publication.
func (e *engine) acceptExtensions() {
	e.acceptedExtensions = map[string]bool{}
	for _, definition := range e.manifest.Providers {
		e.acceptedExtensions[definition.Models], e.acceptedExtensions[definition.Show], e.acceptedExtensions[definition.Chat] = true, true, true
	}
	for _, definition := range e.manifest.Tools {
		e.acceptedExtensions[definition.Callback] = true
	}
}
