package session

import (
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/variables"
)

func TestEvidenceRedactsValuesWithoutCorruptingIdentifiers(t *testing.T) {
	snapshot := variables.NewSnapshot([]variables.Entry{{Name: "TOKEN", Value: "ab"}, {Name: "FIELD", Value: "line"}}, nil)
	e := Evidence{ID: strings.Repeat("ab", 12), Command: "echo ab", Page: &OutputPage{Lines: []OutputLine{{Number: 1, Text: "ab line"}}}}
	rendered := e.Render(snapshot.Redact)
	if !strings.Contains(rendered, e.ID) || !strings.Contains(rendered, `"line":1`) || !strings.Contains(rendered, "wraith::TOKEN wraith::FIELD") {
		t.Fatalf("structural corruption: %s", rendered)
	}
	if e.Page.Lines[0].Text != "ab line" {
		t.Fatal("render mutated evidence")
	}
	original := Session{Messages: []Message{e.Message()}}
	copied := original.Clone()
	copied.Messages[0].Evidence.Page.Lines[0].Text = "changed"
	if original.Messages[0].Evidence.Page.Lines[0].Text != "ab line" {
		t.Fatal("clone shares evidence storage")
	}
}

func TestBackgroundReceiptDoesNotPromiseStartupOrPrivateReads(t *testing.T) {
	for _, hidden := range []bool{false, true} {
		e := Evidence{ID: strings.Repeat("ab", 12), Kind: "started", Command: "printf test", Hidden: hidden}
		text := e.Render(func(s string) string { return s })
		if !strings.Contains(text, "accepted") || !strings.Contains(text, "status: starting") {
			t.Fatal("receipt claimed successful process startup")
		}
		if strings.Contains(text, "poll by id") == hidden {
			t.Fatal("receipt advertised the wrong output policy")
		}
	}
}
