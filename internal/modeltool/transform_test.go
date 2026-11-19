package modeltool

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestArgumentTransformPreservesTypesAndRejectsChangedKeys(t *testing.T) {
	transform := func(value string) (string, error) {
		return strings.ReplaceAll(value, "wraith::TARGET", "example.test"), nil
	}
	input := json.RawMessage(`{"array":[],"object":{},"nested":[null,true,7,"wraith::TARGET"]}`)
	result, err := TransformArguments(input, transform)
	if err != nil || string(result) != `{"array":[],"nested":[null,true,7,"example.test"],"object":{}}` {
		t.Fatalf("argument types changed: %s, %v", result, err)
	}
	if !strings.Contains(string(input), "wraith::TARGET") {
		t.Fatal("input arguments were mutated")
	}
	if _, err := TransformArguments(json.RawMessage(`{"wraith::TARGET":"value"}`), transform); err == nil {
		t.Fatal("argument key was changed")
	}
	for _, replacement := range []string{strings.Repeat("x", 32768), string([]byte{255})} {
		if _, err := TransformArguments(json.RawMessage(`{"value":"x"}`), func(value string) (string, error) {
			if value == "x" {
				return replacement, nil
			}
			return value, nil
		}); err == nil {
			t.Fatal("invalid transformed payload accepted")
		}
	}
}
