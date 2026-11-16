package variables

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRedactKeepsMarkersAndReplacesLongerValuesFirst(t *testing.T) {
	t.Parallel()
	s := NewSnapshot([]Entry{{"DOMAIN", "example.test", "domain example.test"}, {"MAIL", "mail.example.test", "mail"}, {"ALIAS", "DOMAIN", "name"}}, []Entry{{"OLD", "old.test", ""}})
	text := "mail.example.test example.test old.test wraith::DOMAIN DOMAIN"
	want := "wraith::MAIL wraith::DOMAIN wraith::OLD wraith::DOMAIN wraith::ALIAS"
	got := s.Redact(text)
	if got != want || s.Redact(got) != want {
		t.Fatalf("redaction: %s", got)
	}
	inventory, err := s.Inventory()
	if err != nil || strings.Contains(inventory, "example.test") || !strings.Contains(inventory, `"name":"DOMAIN"`) {
		t.Fatalf("inventory: %s, %v", inventory, err)
	}
}

func TestRedactEscapedValuesAndDoesNotModifyCallerEntries(t *testing.T) {
	t.Parallel()
	entries := []Entry{{"TEXT", `a "quoted" value`, ""}}
	s := NewSnapshot(entries, nil)
	entries[0].Value = "changed"
	encoded, _ := json.Marshal(`a "quoted" value`)
	if got := s.Redact(string(encoded)); got != `"wraith::TEXT"` {
		t.Fatal(got)
	}
	if got := s.Redact("a+%22quoted%22+value"); got != "wraith::TEXT" {
		t.Fatal(got)
	}
	if !s.Contains(`a "quoted" value`) {
		t.Fatal("snapshot changed with caller")
	}
}

func TestInventoryChecksSizeAfterRedaction(t *testing.T) {
	t.Parallel()
	s := NewSnapshot([]Entry{{"VERY_LONG_VARIABLE_NAME", "x", strings.Repeat("x", 4096)}}, nil)
	if _, err := s.Inventory(); err == nil {
		t.Fatal("expanded inventory exceeded its budget")
	}
}
