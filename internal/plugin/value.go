package plugin

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/erdajt/wraith/internal/config"
	lua "github.com/yuin/gopher-lua"
)

// decodeValue copies bounded lua values without exposing VM objects to the application.
func decodeValue(value lua.LValue) (any, error) {
	nodes, size := 0, 0
	var visit func(lua.LValue, int) (any, error)
	visit = func(value lua.LValue, depth int) (any, error) {
		nodes++
		if depth > 32 || nodes > 16384 {
			return nil, errors.New("lua value exceeds depth or item limit")
		}
		switch v := value.(type) {
		case *lua.LNilType:
			return nil, nil
		case lua.LBool:
			return bool(v), nil
		case lua.LString:
			size += len(v)
			if size > 1<<20 {
				return nil, errors.New("lua string exceeds 1 mib")
			}
			return string(v), nil
		case lua.LNumber:
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
				return nil, errors.New("lua number must be finite")
			}
			return float64(v), nil
		case *lua.LTable:
			object := map[string]any{}
			array := map[int]any{}
			var problem error
			v.ForEach(func(key, child lua.LValue) {
				if problem != nil {
					return
				}
				var item any
				item, problem = visit(child, depth+1)
				switch k := key.(type) {
				case lua.LString:
					object[string(k)] = item
				case lua.LNumber:
					if k < 1 || k > 16384 || math.Trunc(float64(k)) != float64(k) {
						problem = errors.New("lua array requires consecutive positive indexes")
					} else {
						array[int(k)] = item
					}
				default:
					problem = errors.New("lua table keys must be strings or array indexes")
				}
			})
			if problem != nil {
				return nil, problem
			}
			if len(array) == 0 {
				return object, nil
			}
			if len(object) != 0 {
				return nil, errors.New("lua table mixes object and array keys")
			}
			out := make([]any, len(array))
			for i := range out {
				var ok bool
				if out[i], ok = array[i+1]; !ok {
					return nil, errors.New("lua array has missing indexes")
				}
			}
			return out, nil
		}
		return nil, fmt.Errorf("unsupported lua value: %s", value.Type())
	}
	return visit(value, 0)
}

// encodeValue builds a fresh lua copy of a JSON-compatible application value.
func encodeValue(L *lua.LState, value any) lua.LValue {
	switch v := value.(type) {
	case nil:
		return lua.LNil
	case bool:
		return lua.LBool(v)
	case string:
		return lua.LString(v)
	case float64:
		return lua.LNumber(v)
	case []any:
		table := L.NewTable()
		for _, item := range v {
			table.Append(encodeValue(L, item))
		}
		return table
	case map[string]any:
		table := L.NewTable()
		for key, item := range v {
			table.RawSetString(key, encodeValue(L, item))
		}
		return table
	}
	panic("plugin: value was not normalized through json")
}

// configValue returns a detached configuration object for nested updates and inspection.
func configValue(c config.Config) map[string]any {
	data, _ := json.Marshal(c)
	var object map[string]any
	_ = json.Unmarshal(data, &object)
	if object["themes"] == nil {
		object["themes"] = map[string]any{}
	}
	return object
}

// changeConfig validates a complete candidate after replacing one existing setting path.
func changeConfig(c config.Config, path string, value any) (config.Config, error) {
	object := configValue(c)
	if path == "" {
		var ok bool
		object, ok = value.(map[string]any)
		if !ok {
			return c, errors.New("configuration requires named settings")
		}
	} else {
		parent := object
		parts := strings.Split(path, ".")
		for _, key := range parts[:len(parts)-1] {
			child, ok := parent[key].(map[string]any)
			if !ok {
				return c, fmt.Errorf("unknown setting: %s", path)
			}
			parent = child
		}
		key := parts[len(parts)-1]
		if _, ok := parent[key]; !ok {
			return c, fmt.Errorf("unknown setting: %s", path)
		}
		if empty, ok := value.(map[string]any); ok && len(empty) == 0 {
			if _, array := parent[key].([]any); array {
				value = []any{}
			}
		}
		parent[key] = value
	}
	data, err := json.Marshal(object)
	if err != nil || len(data) > 1<<20 {
		return c, errors.New("configuration exceeds value limits")
	}
	var next config.Config
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&next); err != nil {
		return c, err
	}
	return next, next.Validate()
}
