package variables

import (
	"strings"
	"testing"
)

func TestRewritePreservesCommentsAndOrder(t *testing.T) {
	t.Parallel()
	text := "# detached\n\n// description\nTARGET=old.test /* inline\ncontinued */\nNEXT=value # next\n"
	got, err := Rewrite(text, "TARGET", "new.test", false)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(text, "TARGET=old.test", `TARGET="new.test"`, 1)
	if got != want {
		t.Fatalf("rewrite:\n%s\nwant:\n%s", got, want)
	}
	got, err = Rewrite(got, "TARGET", "", true)
	if err != nil || strings.Contains(got, "new.test") || !strings.Contains(got, "continued */") {
		t.Fatalf("remove: %q, %v", got, err)
	}
	entries, err := Parse(got)
	if err != nil || entries[0].Description != "next" {
		t.Fatalf("deleted description moved to another variable: %+v, %v", entries, err)
	}
}

func TestRewriteRejectsUnsafeNamesAndAddsLiteralValues(t *testing.T) {
	t.Parallel()
	text, err := Rewrite("", "TARGET", "$(not executed) # literal", false)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := Parse(text)
	if err != nil || entries[0].Value != "$(not executed) # literal" {
		t.Fatalf("literal: %+v, %v", entries, err)
	}
	for _, name := range []string{"bad-name", "", "A\nB"} {
		if _, err := Rewrite(text, name, "value", false); err == nil {
			t.Fatal("invalid name accepted")
		}
	}
}
