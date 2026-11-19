package modeltool

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"unicode/utf8"
)

// Definition describes a model-visible extension without exposing runtime callback identities.
type Definition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// Catalog freezes validated schemas so discovery and repair share one accepted contract.
type Catalog struct {
	definitions []Definition
	schemas     map[string]*Schema
}

var toolName = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)

// ValidName excludes native protocol names from extension registration and dispatch.
func ValidName(name string) bool {
	return toolName.MatchString(name) && !slices.Contains([]string{"run", "command", "output", "poll", "request_command", "read_output", "wraith-run", "wraith-tool"}, name)
}

// NewCatalog compiles detached definitions once and rejects ambiguous or oversized catalogs.
func NewCatalog(definitions []Definition) (*Catalog, error) {
	if len(definitions) > 64 {
		return nil, errors.New("tool catalog exceeds 64 definitions")
	}
	catalog := &Catalog{schemas: map[string]*Schema{}}
	for _, definition := range definitions {
		if !ValidName(definition.Name) || !utf8.ValidString(definition.Description) || len(definition.Description) > 4096 {
			return nil, errors.New("tool catalog contains an invalid name or description")
		}
		if catalog.schemas[definition.Name] != nil {
			return nil, fmt.Errorf("duplicate tool definition: %s", definition.Name)
		}
		schema, err := Compile(definition.Parameters)
		if err != nil {
			return nil, fmt.Errorf("tool %s: %w", definition.Name, err)
		}
		definition.Parameters = bytes.Clone(definition.Parameters)
		catalog.definitions = append(catalog.definitions, definition)
		catalog.schemas[definition.Name] = schema
	}
	return catalog, nil
}

// Definitions returns detached wire metadata without exposing compiled schema state.
func (c *Catalog) Definitions() []Definition {
	if c == nil {
		return nil
	}
	definitions := slices.Clone(c.definitions)
	for i := range definitions {
		definitions[i].Parameters = bytes.Clone(definitions[i].Parameters)
	}
	return definitions
}

// Validate accepts arguments only for a schema advertised by this frozen catalog.
func (c *Catalog) Validate(name string, arguments json.RawMessage) error {
	if c == nil || c.schemas[name] == nil {
		return errors.New("unknown extension tool; use an advertised function")
	}
	return c.schemas[name].Validate(arguments)
}

// ValidateArguments checks an extension payload before its selected schema is available.
func ValidateArguments(arguments json.RawMessage) error {
	value, _, err := decode(arguments, 4096)
	if err != nil {
		return err
	}
	if _, ok := value.(map[string]any); !ok {
		return errors.New("tool arguments must be an object")
	}
	return nil
}

// Schema returns the immutable validator bound to an advertised tool name.
func (c *Catalog) Schema(name string) (*Schema, error) {
	if c == nil || c.schemas[name] == nil {
		return nil, errors.New("unknown extension tool; use an advertised function")
	}
	return c.schemas[name], nil
}
