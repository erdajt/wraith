package modeltool

import (
	"strings"
	"testing"
)

func TestDecodeRejectsAmbiguousOrUnboundedJSON(t *testing.T) {
	for name, raw := range map[string]string{
		"duplicate":         `{"name":1,"name":2}`,
		"escaped duplicate": `{"name":1,"\u006eame":2}`,
		"nested duplicate":  `{"a":[{"x":0,"x":1}]}`,
		"invalid utf8":      "{\"x\":\"\xff\"}",
		"high surrogate":    `{"x":"\ud800"}`,
		"low surrogate":     `{"x":"\udc00"}`,
		"wrong pair":        `{"x":"\ud800\u0041"}`,
		"deep":              strings.Repeat("[", 17) + "0" + strings.Repeat("]", 17),
		"nodes":             `[` + strings.Repeat("0,", 4096) + `0]`,
		"bytes":             `"` + strings.Repeat("x", maxBytes) + `"`,
		"trailing":          `{} []`,
		"incomplete":        `{"x":`,
		"number":            `1e999`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := decode([]byte(raw), 4096); err == nil {
				t.Fatal("invalid input accepted")
			}
		})
	}
}

func TestDecodePreservesValidUnicodeAndCountsKeys(t *testing.T) {
	value, size, err := decode([]byte(`{"\ud83d\udc7b":"é","a":[null,true,1.5,"\\ud800"]}`), 32)
	if err != nil {
		t.Fatal(err)
	}
	object := value.(map[string]any)
	if object["👻"] != "é" || size.nodes != 9 || size.text != 13 {
		t.Fatalf("value=%v size=%+v", value, size)
	}
	if _, _, err := decode([]byte(`{"a":1}`), 2); err == nil {
		t.Fatal("object key did not count toward node limit")
	}
}
