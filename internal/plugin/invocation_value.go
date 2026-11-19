package plugin

import (
	"bytes"
	"encoding/json"
	"errors"
	"unicode/utf8"
)

// invocationPayload validates detached wire values before they enter a retained queue or Lua.
func invocationPayload(data json.RawMessage) (json.RawMessage, error) {
	if len(data) == 0 {
		return json.RawMessage("null"), nil
	}
	if len(data) > 1<<20 || !utf8.Valid(data) {
		return nil, errors.New("invocation payload exceeds 1 mib or contains invalid utf8")
	}
	if err := validateJSONEscapes(string(data)); err != nil {
		return nil, err
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, errors.New("invalid invocation json payload")
	}
	nodes := 0
	var visit func(any, int) bool
	visit = func(value any, depth int) bool {
		nodes++
		if nodes > 16384 || depth > 32 {
			return false
		}
		switch value := value.(type) {
		case []any:
			for _, child := range value {
				if !visit(child, depth+1) {
					return false
				}
			}
		case map[string]any:
			for _, child := range value {
				if !visit(child, depth+1) {
					return false
				}
			}
		}
		return true
	}
	if !visit(value, 0) {
		return nil, errors.New("invocation payload exceeds depth or item limits")
	}
	return bytes.Clone(data), nil
}
