package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"unicode/utf8"

	lua "github.com/yuin/gopher-lua"
)

type jsonCodec struct {
	L             *lua.LState
	null          *lua.LUserData
	array, object *lua.LTable
}

type jsonBudget struct {
	nodes, bytes int
	active       map[*lua.LTable]bool
}

// installJSON exposes bounded JSON values without erasing nulls or empty container types.
func (e *engine) installJSON(api *lua.LTable) {
	L := e.L
	codec := jsonCodec{L: L, null: L.NewUserData(), array: L.NewTable(), object: L.NewTable()}
	e.jsonCodec = codec
	codec.array.RawSetString("__metatable", lua.LFalse)
	codec.object.RawSetString("__metatable", lua.LFalse)
	methods := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"encode": func(L *lua.LState) int {
			value, err := codec.toGo(L.Get(1), 0, &jsonBudget{active: map[*lua.LTable]bool{}})
			if err != nil {
				L.RaiseError("json.encode: %v", err)
			}
			data, err := json.Marshal(value)
			if err != nil || len(data) > 1<<20 {
				L.RaiseError("json.encode: result exceeds 1 mib or cannot be encoded")
			}
			L.Push(lua.LString(data))
			return 1
		},
		"decode": func(L *lua.LState) int {
			input := L.CheckString(1)
			if len(input) > 1<<20 || !utf8.ValidString(input) {
				L.RaiseError("json.decode: input requires valid UTF-8 within 1 mib")
			}
			var value any
			if err := json.Unmarshal([]byte(input), &value); err != nil {
				L.RaiseError("json.decode: %v", err)
			}
			if err := validateJSONEscapes(input); err != nil {
				L.RaiseError("json.decode: %v", err)
			}
			decoded, err := codec.toLua(value, 0, &jsonBudget{})
			if err != nil {
				L.RaiseError("json.decode: %v", err)
			}
			L.Push(decoded)
			return 1
		},
	})
	for name, kind := range map[string]*lua.LTable{"array": codec.array, "object": codec.object} {
		methods.RawSetString(name, L.NewFunction(func(L *lua.LState) int {
			table := L.OptTable(1, L.NewTable())
			L.SetMetatable(table, kind)
			L.Push(table)
			return 1
		}))
	}
	methods.RawSetString("null", codec.null)
	api.RawSetString("json", methods)
}

// check limits traversal before descending into another JSON value.
func (b *jsonBudget) check(depth, size int) error {
	b.nodes++
	b.bytes += size
	if depth > 32 || b.nodes > 16384 || b.bytes > 1<<20 {
		return errors.New("value exceeds depth, item or byte limit")
	}
	return nil
}

// toGo copies Lua tables while preserving explicit container kinds and rejecting lossy strings.
func (c jsonCodec) toGo(value lua.LValue, depth int, budget *jsonBudget) (any, error) {
	size := 0
	if text, ok := value.(lua.LString); ok {
		size = len(text)
	}
	if err := budget.check(depth, size); err != nil {
		return nil, err
	}
	if value == c.null {
		return nil, nil
	}
	switch v := value.(type) {
	case *lua.LNilType:
		return nil, errors.New("use json.null instead of nil")
	case lua.LBool:
		return bool(v), nil
	case lua.LString:
		if !utf8.ValidString(string(v)) {
			return nil, errors.New("string is not valid UTF-8")
		}
		return string(v), nil
	case lua.LNumber:
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return nil, errors.New("number must be finite")
		}
		return float64(v), nil
	case *lua.LTable:
		if budget.active[v] {
			return nil, errors.New("cyclic table")
		}
		budget.active[v] = true
		defer delete(budget.active, v)
		object, array := map[string]any{}, map[int]any{}
		var problem error
		v.ForEach(func(key, child lua.LValue) {
			if problem != nil {
				return
			}
			var item any
			item, problem = c.toGo(child, depth+1, budget)
			if problem != nil {
				return
			}
			switch k := key.(type) {
			case lua.LString:
				if v.Metatable == c.array || !utf8.ValidString(string(k)) {
					problem = errors.New("object keys require valid UTF-8 and an object table")
					return
				}
				problem = budget.check(depth+1, len(k))
				object[string(k)] = item
			case lua.LNumber:
				if v.Metatable == c.object || k < 1 || k > 16384 || math.Trunc(float64(k)) != float64(k) {
					problem = errors.New("array keys require consecutive positive integers")
					return
				}
				array[int(k)] = item
			default:
				problem = errors.New("table keys require strings or array indexes")
			}
		})
		if problem != nil {
			return nil, problem
		}
		if len(array) > 0 && len(object) > 0 {
			return nil, errors.New("table mixes array and object keys")
		}
		if len(array) == 0 && v.Metatable != c.array {
			return object, nil
		}
		out := make([]any, len(array))
		for i := range out {
			var exists bool
			out[i], exists = array[i+1]
			if !exists {
				return nil, errors.New("array has missing indexes")
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("unsupported value: %s", value.Type())
}

// toLua restores null sentinels and marks decoded containers for lossless re-encoding.
func (c jsonCodec) toLua(value any, depth int, budget *jsonBudget) (lua.LValue, error) {
	size := 0
	if text, ok := value.(string); ok {
		size = len(text)
	}
	if err := budget.check(depth, size); err != nil {
		return nil, err
	}
	switch v := value.(type) {
	case nil:
		return c.null, nil
	case bool:
		return lua.LBool(v), nil
	case float64:
		return lua.LNumber(v), nil
	case string:
		return lua.LString(v), nil
	case []any:
		table := c.L.NewTable()
		table.Metatable = c.array
		for i, item := range v {
			child, err := c.toLua(item, depth+1, budget)
			if err != nil {
				return nil, err
			}
			table.RawSetInt(i+1, child)
		}
		return table, nil
	case map[string]any:
		table := c.L.NewTable()
		table.Metatable = c.object
		for key, item := range v {
			if err := budget.check(depth+1, len(key)); err != nil {
				return nil, err
			}
			child, err := c.toLua(item, depth+1, budget)
			if err != nil {
				return nil, err
			}
			table.RawSetString(key, child)
		}
		return table, nil
	}
	return nil, errors.New("unsupported decoded value")
}
