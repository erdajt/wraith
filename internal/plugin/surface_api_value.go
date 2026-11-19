package plugin

import (
	"bytes"
	"encoding/json"
	"reflect"
	"unicode/utf8"

	"github.com/erdajt/wraith/internal/keymap"
	lua "github.com/yuin/gopher-lua"
)

// decodeTypedValue copies strict typed options and preserves empty arrays using their destination types.
func decodeTypedValue(L *lua.LState, value lua.LValue, destination any) {
	copied, err := decodeValue(value)
	if err != nil {
		L.RaiseError("typed options: %v", err)
	}
	size := 0
	var validate func(any)
	validate = func(value any) {
		switch value := value.(type) {
		case string:
			size += len(value)
			if size > 1<<20 {
				L.RaiseError("typed options exceed 1 mib")
			}
			if !utf8.ValidString(value) {
				L.RaiseError("typed options require valid utf-8; use ui.decode for binary input")
			}
		case []any:
			for _, item := range value {
				validate(item)
			}
		case map[string]any:
			for key, item := range value {
				validate(key)
				validate(item)
			}
		}
	}
	validate(copied)
	copied = normalizeTypedArrays(copied, reflect.TypeOf(destination).Elem())
	data, err := json.Marshal(copied)
	if err != nil {
		L.RaiseError("typed options: %v", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		L.RaiseError("typed options: %v", err)
	}
}

// pushSurface returns a detached typed value without exposing Go or Lua-owned storage.
func pushSurface(L *lua.LState, value any) int {
	switch item := value.(type) {
	case SurfaceBuffer:
		item.Spans = append([]SurfaceSpan{}, item.Spans...)
		item.Keymaps = append([]keymap.Binding{}, item.Keymaps...)
		value = item
	case SurfaceRegion:
		item.Lines = append([]string{}, item.Lines...)
		item.Spans = append([]SurfaceSpan{}, item.Spans...)
		value = item
	case SurfaceBufferPage:
		item.Lines = append([]string{}, item.Lines...)
		value = item
	}
	if reflected := reflect.ValueOf(value); reflected.IsValid() && reflected.Kind() == reflect.Slice && reflected.IsNil() {
		value = []any{}
	}
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
	decodeTypedValue(L, L.Get(index), &value)
	if value > 1<<53 || value < -(1<<53) {
		L.RaiseError("surface index exceeds exact integer range")
	}
	return value
}
