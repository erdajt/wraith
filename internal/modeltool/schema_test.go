package modeltool

import (
	"fmt"
	"strings"
	"testing"
)

func TestSchemaRejectsRoundedIntegerAliases(t *testing.T) {
	if _, err := Compile([]byte(`{"type":"object","properties":{"id":{"enum":[9007199254740993]}}}`)); err == nil {
		t.Fatal("schema accepted an integer that rounds to another identifier")
	}
	schema, err := Compile([]byte(`{"type":"object","properties":{"id":{"type":"integer"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate([]byte(`{"id":9007199254740993}`)); err == nil {
		t.Fatal("arguments accepted a rounded identifier")
	}
	stringsOnly, err := Compile([]byte(`{"type":"object","properties":{"id":{"enum":["9007199254740993"]}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := stringsOnly.Validate([]byte(`{"id":"9007199254740992"}`)); err == nil {
		t.Fatal("distinct string identifiers matched")
	}
	if err := stringsOnly.Validate([]byte(`{"id":"9007199254740993"}`)); err != nil {
		t.Fatal(err)
	}
}

func TestSchemaValidatesNestedToolArguments(t *testing.T) {
	schema, err := Compile([]byte(`{
		"type":"object", "required":["command","options"], "additionalProperties":false,
		"properties":{
			"command":{"type":"string","minLength":1,"maxLength":40,"pattern":"^[a-z ]+$"},
			"options":{"type":"object","required":["limit"],"additionalProperties":false,
				"properties":{"limit":{"type":"integer","minimum":1,"maximum":10},"format":{"enum":["text","json"]}}}
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate([]byte(`{"command":"show status","options":{"limit":3,"format":"json"}}`)); err != nil {
		t.Fatal(err)
	}
	for name, raw := range map[string]string{
		"missing":    `{"command":"show"}`,
		"extra":      `{"command":"show","options":{"limit":3},"hidden":true}`,
		"fractional": `{"command":"show","options":{"limit":1.5}}`,
		"range":      `{"command":"show","options":{"limit":11}}`,
		"pattern":    `{"command":"show; secret","options":{"limit":1}}`,
		"enum":       `{"command":"show","options":{"limit":1,"format":"secret"}}`,
		"duplicate":  `{"command":"show","command":"secret","options":{"limit":1}}`,
		"root":       `[]`,
	} {
		t.Run(name, func(t *testing.T) {
			if err := schema.Validate([]byte(raw)); err == nil || strings.Contains(err.Error(), "secret") {
				t.Fatalf("invalid arguments accepted or disclosed: %v", err)
			}
		})
	}
}

func TestSchemaSupportsConditionalAndArrayValidation(t *testing.T) {
	schema, err := Compile([]byte(`{"type":"object","required":["mode","values"],
		"properties":{"mode":{"enum":["one","many"]},"values":{"type":"array","items":{"type":"integer"},"uniqueItems":true,"contains":{"minimum":2}}},
		"if":{"properties":{"mode":{"const":"one"}}},
		"then":{"properties":{"values":{"maxItems":1}}},
		"else":{"properties":{"values":{"minItems":2}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{`{"mode":"one","values":[2]}`, `{"mode":"many","values":[1,2]}`} {
		if err := schema.Validate([]byte(raw)); err != nil {
			t.Fatal(err)
		}
	}
	for _, raw := range []string{`{"mode":"one","values":[1,2]}`, `{"mode":"many","values":[2,2]}`, `{"mode":"one","values":[1]}`} {
		if err := schema.Validate([]byte(raw)); err == nil {
			t.Fatal("invalid conditional arguments accepted")
		}
	}
}

func TestSchemaRejectsUnsupportedOrUnboundedDefinitions(t *testing.T) {
	for name, raw := range map[string]string{
		"root":                  `{"type":"array"}`,
		"draft":                 `{"type":"object","$schema":"http://json-schema.org/draft-07/schema#"}`,
		"reference":             `{"type":"object","properties":{"x":{"$ref":"#"}}}`,
		"external":              `{"type":"object","$ref":"https://example.invalid/schema"}`,
		"dynamic":               `{"type":"object","$dynamicRef":"#node"}`,
		"format":                `{"type":"object","properties":{"x":{"format":"email"}}}`,
		"content":               `{"type":"object","contentEncoding":"base64"}`,
		"unknown":               `{"type":"object","typo":true}`,
		"invalid child":         `{"type":"object","properties":{"x":[]}}`,
		"invalid branch":        `{"type":"object","allOf":{}}`,
		"invalid keyword value": `{"type":"object","required":true}`,
		"invalid regex":         `{"type":"object","patternProperties":{"[":true}}`,
		"regex bytes":           fmt.Sprintf(`{"type":"object","patternProperties":{%q:true}}`, strings.Repeat("a", 1025)),
		"large repeated group":  fmt.Sprintf(`{"type":"object","patternProperties":{%q:true}}`, "("+strings.Repeat("a", 900)+"){1000}"),
		"expanded regex":        `{"type":"object","patternProperties":{"a{1000}b{1000}c{1000}d{1000}e{1000}":true}}`,
		"aggregate regex":       `{"type":"object","allOf":[` + strings.TrimSuffix(strings.Repeat(fmt.Sprintf(`{"pattern":%q},`, strings.Repeat("a", 1024)), 5), ",") + `]}`,
		"nodes":                 `{"type":"object","allOf":[` + strings.Repeat(`{},`, 510) + `{}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Compile([]byte(raw)); err == nil {
				t.Fatal("invalid schema accepted")
			}
		})
	}
}

func TestSchemaBoundsCombinedWorkAndAllowsAnnotationData(t *testing.T) {
	schema, err := Compile([]byte(fmt.Sprintf(`{"type":"object","properties":{"value":{"pattern":%q}},"default":{"$ref":"ordinary data"}}`, strings.Repeat("a?", 512))))
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate([]byte(`{"value":"a"}`)); err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate([]byte(`{"value":"` + strings.Repeat("a", 16384) + `"}`)); err == nil || !strings.Contains(err.Error(), "work limit") {
		t.Fatalf("expensive combined input was not rejected: %v", err)
	}
	var unavailable *Schema
	if err := unavailable.Validate([]byte(`{}`)); err == nil {
		t.Fatal("nil schema accepted arguments")
	}
}

func TestSchemaBoundsRepeatedArrayValidation(t *testing.T) {
	definition := `{"type":"object","properties":{"values":{"allOf":[` + strings.TrimSuffix(strings.Repeat(`{"items":{"type":"integer"}},`, 70), ",") + `]}}}`
	schema, err := Compile([]byte(definition))
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate([]byte(`{"values":[1,2,3]}`)); err != nil {
		t.Fatal(err)
	}
	arguments := []byte(`{"values":[` + strings.Repeat("1,", 3999) + `1]}`)
	if err := schema.Validate(arguments); err == nil || !strings.Contains(err.Error(), "work limit") {
		t.Fatalf("repeated evaluation accepted: %v", err)
	}
}

func TestCompiledSchemaSupportsConcurrentValidation(t *testing.T) {
	schema, err := Compile([]byte(`{"type":"object","properties":{"x":{"type":"integer"}},"additionalProperties":false}`))
	if err != nil {
		t.Fatal(err)
	}
	for i := range 16 {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			for range 20 {
				if err := schema.Validate([]byte(`{"x":1}`)); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkSchemaValidation(b *testing.B) {
	for _, fixture := range []struct{ name, schema, arguments string }{
		{"command", `{"type":"object","properties":{"command":{"type":"string"}},"required":["command"]}`, `{"command":"printf hello"}`},
		{"branches", `{"type":"object","properties":{"values":{"allOf":[` + strings.TrimSuffix(strings.Repeat(`{"items":{"type":"integer"}},`, 8), ",") + `]}}}`, `{"values":[` + strings.Repeat("1,", 3999) + `1]}`},
		{"regex", `{"type":"object","properties":{"value":{"pattern":"^(a?){64}a*$"}}}`, `{"value":"` + strings.Repeat("a", 16000) + `"}`},
	} {
		b.Run(fixture.name, func(b *testing.B) {
			schema, err := Compile([]byte(fixture.schema))
			if err != nil {
				b.Fatal(err)
			}
			arguments := []byte(fixture.arguments)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if err := schema.Validate(arguments); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkSchemaRejectsRegexExpansion(b *testing.B) {
	definition := []byte(fmt.Sprintf(`{"type":"object","patternProperties":{%q:true}}`, "("+strings.Repeat("a", 900)+"){1000}"))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Compile(definition); err == nil {
			b.Fatal("oversized expanded regex accepted")
		}
	}
}
