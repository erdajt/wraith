package plugin

import (
	"bytes"
	"encoding/json"
	"reflect"

	lua "github.com/yuin/gopher-lua"
)

// decodeSurface copies strict typed options and accepts empty Lua arrays at known sequence fields.
func decodeSurface(L *lua.LState, value lua.LValue, destination any) {
	copied, err := decodeValue(value)
	if err != nil {
		L.RaiseError("surface options: %v", err)
	}
	if object, ok := copied.(map[string]any); ok {
		if len(object) == 0 && reflect.TypeOf(destination).Elem().Kind() == reflect.Slice {
			copied = []any{}
		} else {
			for _, key := range []string{"lines", "spans", "keymaps"} {
				if empty, ok := object[key].(map[string]any); ok && len(empty) == 0 {
					object[key] = []any{}
				}
			}
		}
	}
	data, err := json.Marshal(copied)
	if err != nil {
		L.RaiseError("surface options: %v", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		L.RaiseError("surface options: %v", err)
	}
}

// pushSurface returns a detached typed value without exposing Go or Lua-owned storage.
func pushSurface(L *lua.LState, value any) int {
	data, err := json.Marshal(value)
	if err != nil {
		L.RaiseError("surface value: %v", err)
	}
	var normalized any
	if err := json.Unmarshal(data, &normalized); err != nil {
		L.RaiseError("surface value: %v", err)
	}
	L.Push(encodeValue(L, normalized))
	return 1
}

// surfaceInteger rejects coercion and fractional indexes before they reach line operations.
func surfaceInteger(L *lua.LState, index, fallback int) int {
	if L.Get(index) == lua.LNil {
		return fallback
	}
	var value int
	decodeSurface(L, L.Get(index), &value)
	if value > 1<<53 || value < -(1<<53) {
		L.RaiseError("surface index exceeds exact integer range")
	}
	return value
}
