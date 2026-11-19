package modeltool

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/erdajt/wraith/internal/regexutil"

	"github.com/google/jsonschema-go/jsonschema"
)

const draft = "https://json-schema.org/draft/2020-12/schema"
const maxWork = 8 << 20

// Schema retains a resolved reference-free tool argument schema.
type Schema struct {
	resolved *jsonschema.Resolved
	weight   int
	nodes    int
}

// Compile validates a bounded object schema without resolving external references.
func Compile(raw []byte) (*Schema, error) {
	value, size, err := decode(raw, 512)
	if err != nil {
		return nil, fmt.Errorf("tool schema: %w", err)
	}
	root, ok := value.(map[string]any)
	if !ok || root["type"] != "object" {
		return nil, errors.New("tool schema must have an object root type")
	}
	patterns := regexSize{}
	if err := checkSchema(root, &patterns); err != nil {
		return nil, err
	}

	var schema jsonschema.Schema
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil, errors.New("tool schema has invalid keyword values")
	}
	schema.Schema = draft
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return nil, fmt.Errorf("invalid tool schema: %w", err)
	}
	return &Schema{resolved: resolved, weight: size.nodes + patterns.instructions, nodes: size.nodes}, nil
}

// Validate checks bounded arguments without including their contents in errors.
func (s *Schema) Validate(raw []byte) error {
	if s == nil || s.resolved == nil {
		return errors.New("tool schema is unavailable")
	}
	value, size, err := decode(raw, 4096)
	if err != nil {
		return fmt.Errorf("tool arguments: %w", err)
	}
	if _, ok := value.(map[string]any); !ok {
		return errors.New("tool arguments must be an object")
	}
	if size.nodes > (256<<10)/s.nodes || size.nodes+size.text > maxWork/s.weight {
		return errors.New("tool schema and arguments exceed validation work limit")
	}
	if err := s.resolved.Validate(value); err != nil {
		return errors.New("tool arguments do not match schema")
	}
	return nil
}

// checkSchema restricts schema locations while allowing arbitrary property names and data values.
func checkSchema(value any, patterns *regexSize) error {
	if _, ok := value.(bool); ok {
		return nil
	}
	object, ok := value.(map[string]any)
	if !ok {
		return errors.New("tool subschema must be an object or boolean")
	}
	for key, value := range object {
		switch key {
		case "$schema":
			if value != draft {
				return errors.New("tool schema must use draft 2020-12")
			}
		case "type", "enum", "const", "title", "description", "$comment", "default", "examples", "deprecated", "readOnly", "writeOnly",
			"required", "dependentRequired", "minProperties", "maxProperties", "minItems", "maxItems", "minContains", "maxContains", "uniqueItems",
			"minLength", "maxLength", "minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum", "multipleOf":
		case "pattern":
			pattern, ok := value.(string)
			if !ok {
				return errors.New("tool schema pattern must be a string")
			}
			if err := countPattern(pattern, patterns); err != nil {
				return err
			}
		case "properties", "patternProperties", "dependentSchemas":
			children, ok := value.(map[string]any)
			if !ok {
				return errors.New("tool schema property map must be an object")
			}
			for name, child := range children {
				if key == "patternProperties" {
					if err := countPattern(name, patterns); err != nil {
						return err
					}
				}
				if err := checkSchema(child, patterns); err != nil {
					return err
				}
			}
		case "allOf", "anyOf", "oneOf", "prefixItems":
			children, ok := value.([]any)
			if !ok {
				return errors.New("tool schema branches must be an array")
			}
			for _, child := range children {
				if err := checkSchema(child, patterns); err != nil {
					return err
				}
			}
		case "additionalProperties", "unevaluatedProperties", "propertyNames", "items", "unevaluatedItems", "contains", "not", "if", "then", "else":
			if err := checkSchema(value, patterns); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported tool schema keyword: %s", key)
		}
	}
	return nil
}

type regexSize struct {
	bytes        int
	instructions int
}

// countPattern bounds regex source and expanded programs before schema validation.
func countPattern(pattern string, total *regexSize) error {
	total.bytes += len(pattern)
	if len(pattern) > 1024 || total.bytes > 4096 {
		return errors.New("tool schema exceeds regex size limit")
	}
	size, err := regexutil.Size(pattern)
	if err != nil {
		return err
	}
	total.instructions += size
	if total.instructions > 8192 {
		return errors.New("tool schema exceeds expanded regex limit")
	}
	return nil
}
