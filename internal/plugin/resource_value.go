package plugin

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"

	lua "github.com/yuin/gopher-lua"
)

// cloneResourceValue detaches bounded worker data while preserving arbitrary byte strings.
func cloneResourceValue(value any) (any, error) {
	nodes, bytes := 0, 0
	var copy func(any, int) (any, error)
	copy = func(value any, depth int) (any, error) {
		nodes++
		if depth > 32 || nodes > 100000 || bytes > 8<<20 {
			return nil, errors.New("resource value exceeds size or depth limits")
		}
		switch value := value.(type) {
		case nil, bool:
			return value, nil
		case string:
			bytes += len(value)
			if bytes > 8<<20 {
				return nil, errors.New("resource value exceeds 8 mib")
			}
			return value, nil
		case []byte:
			bytes += len(value)
			if bytes > 8<<20 {
				return nil, errors.New("resource value exceeds 8 mib")
			}
			return slices.Clone(value), nil
		case []string:
			items := make([]any, len(value))
			for i, item := range value {
				items[i] = item
			}
			return copy(items, depth+1)
		case []any:
			items := make([]any, len(value))
			for i, item := range value {
				detached, err := copy(item, depth+1)
				if err != nil {
					return nil, err
				}
				items[i] = detached
			}
			return items, nil
		case map[string]any:
			items := make(map[string]any, len(value))
			for key, item := range value {
				bytes += len(key)
				detached, err := copy(item, depth+1)
				if err != nil {
					return nil, err
				}
				items[key] = detached
			}
			return items, nil
		}
		number := reflect.ValueOf(value)
		var result float64
		switch number.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			integer := number.Int()
			if integer > 1<<53 || integer < -(1<<53) {
				return nil, errors.New("resource integer exceeds exact lua number range")
			}
			result = float64(integer)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			integer := number.Uint()
			if integer > 1<<53 {
				return nil, errors.New("resource integer exceeds exact lua number range")
			}
			result = float64(integer)
		case reflect.Float32, reflect.Float64:
			result = number.Float()
		default:
			return nil, fmt.Errorf("unsupported resource value: %T", value)
		}
		if math.IsInf(result, 0) || math.IsNaN(result) {
			return nil, errors.New("resource number must be finite")
		}
		return result, nil
	}
	return copy(value, 0)
}

// encodeResourceValue creates Lua values without routing binary payloads through JSON text.
func encodeResourceValue(L *lua.LState, value any) (lua.LValue, error) {
	detached, err := cloneResourceValue(value)
	if err != nil {
		return nil, err
	}
	var encode func(any) lua.LValue
	encode = func(value any) lua.LValue {
		switch value := value.(type) {
		case []byte:
			return lua.LString(string(value))
		case []any:
			table := L.NewTable()
			for _, item := range value {
				table.Append(encode(item))
			}
			return table
		case map[string]any:
			table := L.NewTable()
			for key, item := range value {
				table.RawSetString(key, encode(item))
			}
			return table
		default:
			return encodeValue(L, value)
		}
	}
	return encode(detached), nil
}
