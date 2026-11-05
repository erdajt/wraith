package markdown

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/erdajt/wraith/internal/config"
)

func TestInlineMarkdownPreservesTextAndAppliesAttributes(t *testing.T) {
	p := config.Default().Palette()
	for _, tt := range []struct{ input, plain, attribute string }{
		{"**bold**", "bold", ";1m"},
		{"*italic*", "italic", ";3m"},
		{"~~old~~", "old", ";9m"},
		{"**bold and *nested***", "bold and nested", ";1;3m"},
		{"`a **literal**`", "a **literal**", rgb(p.Accent)},
		{"[docs](https://example.org)", "docs (https://example.org)", ";4m"},
		{"a_variable_name", "a_variable_name", ""},
		{"\\*literal\\*", "*literal*", ""},
		{"界 **wide**", "界 wide", ";1m"},
	} {
		t.Run(tt.input, func(t *testing.T) {
			got := inline(tt.input, p, "", 0)
			if ansi.Strip(got) != tt.plain || !strings.Contains(got, tt.attribute) {
				t.Fatalf("got %q", got)
			}
		})
	}
}

func TestIncompleteMarkupRemainsReadable(t *testing.T) {
	p := config.Default().Palette()
	for _, text := range []string{"**pending", "`pending", "[pending", strings.Repeat("*", 100)} {
		if ansi.Strip(inline(text, p, "", 0)) == "" {
			t.Fatalf("lost streaming text %q", text)
		}
	}
}
