package modeltool

import (
	"encoding/json"
	"errors"
	"unicode/utf8"
)

// TransformArguments replaces string values while preserving argument keys and JSON types.
func TransformArguments(arguments json.RawMessage, transform func(string) (string, error)) (json.RawMessage, error) {
	value, _, err := decode(arguments, 4096)
	if err != nil {
		return nil, err
	}
	if _, ok := value.(map[string]any); !ok {
		return nil, errors.New("tool arguments must be an object")
	}
	var visit func(any) (any, error)
	visit = func(value any) (any, error) {
		switch value := value.(type) {
		case string:
			result, err := transform(value)
			if err != nil {
				return nil, err
			}
			if !utf8.ValidString(result) {
				return nil, errors.New("transformed tool argument is not valid utf-8")
			}
			return result, nil
		case []any:
			for i, child := range value {
				next, err := visit(child)
				if err != nil {
					return nil, err
				}
				value[i] = next
			}
		case map[string]any:
			for key, child := range value {
				nextKey, err := transform(key)
				if err != nil {
					return nil, err
				}
				if nextKey != key {
					return nil, errors.New("private values and references cannot change tool argument keys")
				}
				next, err := visit(child)
				if err != nil {
					return nil, err
				}
				value[key] = next
			}
		}
		return value, nil
	}
	value, err = visit(value)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if err := ValidateArguments(encoded); err != nil {
		return nil, err
	}
	return encoded, nil
}
