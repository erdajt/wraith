package markdown

import (
	"github.com/charmbracelet/x/ansi"
	"github.com/erdajt/wraith/internal/config"
	"strings"
	"testing"
)

func TestBlocksRenderWithoutMarkdownDelimiters(t *testing.T) {
	source := "# **heading**\n\n- item\n- [x] done\n> quoted\n\n````sh\nprintf '**literal**'\n```\n````\n"
	lines, blocks := Render(source, 50, config.Default().Palette(), 0)
	text := ansi.Strip(strings.Join(lines, "\n"))
	for _, want := range []string{"heading", "• item", "☑ done", "│ quoted", "printf '**literal**'", "│ ```"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
	if blocks != 1 || strings.Contains(text, "# **heading**") {
		t.Fatal("markdown syntax leaked")
	}
}

func TestTablesFitAndRetainValuesAtNarrowWidths(t *testing.T) {
	source := "| host | status |\n| --- | --- |\n| lab.internal | **open** |\n| 界 | closed |"
	for _, width := range []int{12, 80} {
		lines, _ := Render(source, width, config.Default().Palette(), 0)
		text := ansi.Strip(strings.Join(lines, "\n"))
		for _, line := range lines {
			if ansi.StringWidth(line) > width {
				t.Fatal("table overflow")
			}
		}
		if !strings.Contains(text, "closed") || !strings.Contains(text, "open") || !strings.Contains(text, "界") {
			t.Fatal("table content lost")
		}
		if strings.Contains(text, "**") || strings.Contains(text, "| ---") {
			t.Fatal("raw table delimiters")
		}
	}
}
