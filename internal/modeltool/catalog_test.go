package modeltool

import (
	"encoding/json"
	"testing"
)

func TestCatalogDetachesMetadataAndRetainsCompiledSchema(t *testing.T) {
	definitions := []Definition{{Name: "inspect_file", Description: "inspect metadata", Parameters: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"],"additionalProperties":false}`)}}
	catalog, err := NewCatalog(definitions)
	if err != nil {
		t.Fatal(err)
	}
	definitions[0].Parameters[0] = '!'
	metadata := catalog.Definitions()
	metadata[0].Parameters[0] = '?'
	metadata[0].Name = "changed"
	if got := catalog.Definitions(); got[0].Name != "inspect_file" || got[0].Parameters[0] != '{' {
		t.Fatal("catalog retained caller-owned metadata")
	}
	if err := catalog.Validate("inspect_file", json.RawMessage(`{"path":"wraith::TARGET"}`)); err != nil {
		t.Fatal(err)
	}
	for _, arguments := range []string{`{}`, `{"path":4}`, `{"path":"one","path":"two"}`, `{"path":"x","extra":true}`} {
		if err := catalog.Validate("inspect_file", json.RawMessage(arguments)); err == nil {
			t.Fatalf("invalid arguments accepted: %s", arguments)
		}
	}
	if err := catalog.Validate("missing", json.RawMessage(`{}`)); err == nil {
		t.Fatal("unknown tool accepted")
	}
}

func TestCatalogRejectsReservedRepeatedAndInvalidDefinitions(t *testing.T) {
	valid := Definition{Name: "metadata", Parameters: json.RawMessage(`{"type":"object"}`)}
	for _, name := range []string{"request_command", "read_output", "wraith-run", "wraith-tool", "run", "output", "poll", "UPPER", "bad name", ""} {
		definition := valid
		definition.Name = name
		if _, err := NewCatalog([]Definition{definition}); err == nil {
			t.Fatalf("invalid tool name accepted: %q", name)
		}
	}
	if _, err := NewCatalog([]Definition{valid, valid}); err == nil {
		t.Fatal("duplicate tool catalog accepted")
	}
	invalid := valid
	invalid.Parameters = json.RawMessage(`{"type":"object","$ref":"https://example.test/schema"}`)
	if _, err := NewCatalog([]Definition{invalid}); err == nil {
		t.Fatal("unvalidated schema entered catalog")
	}
	var absent *Catalog
	if len(absent.Definitions()) != 0 || absent.Validate("metadata", json.RawMessage(`{}`)) == nil {
		t.Fatal("absent catalog advertised or accepted tools")
	}
}
