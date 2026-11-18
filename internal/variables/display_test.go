package variables

import (
	"reflect"
	"testing"
)

func TestLiteralSpansPreserveEscapedValuesAndLongestMatches(t *testing.T) {
	t.Parallel()
	s := NewSnapshot([]Entry{{Name: "HOST", Value: "lab.example"}, {Name: "DOMAIN", Value: "example"}, {Name: "QUOTE", Value: "a'b"}}, nil)
	text := "λ lab.example 'a'\\''b' example"
	var values []string
	for _, span := range s.LiteralSpans(text) {
		values = append(values, string([]rune(text)[span.Start:span.End]))
	}

	if !reflect.DeepEqual(values, []string{"lab.example", "a'\\''b", "example"}) {
		t.Fatalf("unexpected display spans: %q", values)
	}
}
