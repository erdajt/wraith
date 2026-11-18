package markdown

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/erdajt/wraith/internal/config"
)

func TestWrappedRowsRetainSourceLineAndDisplayOffset(t *testing.T) {
	source := "same line\n" + strings.Repeat("界ab", 20) + "\nsame line\n```sh\nprintf 'hello world'\n```"
	for _, width := range []int{12, 40, 100} {
		rows, _ := Render(source, width, config.Default().Palette(), 0)
		cell := 0
		seen := map[int]bool{}
		for _, row := range rows {
			seen[row.Source] = true
			if row.Source == 1 {
				if row.Cell != cell {
					t.Fatalf("width %d: cell %d, want %d", width, row.Cell, cell)
				}
				cell += ansi.StringWidth(row.Text)
			}
			if row.Source == 4 && row.Block != 0 {
				t.Fatal("command lost its block identity")
			}
		}
		if cell != 80 || !seen[0] || !seen[2] || !seen[4] {
			t.Fatalf("source positions lost: %+v, cells %d", seen, cell)
		}
	}
}

func TestTableReflowRetainsSourceRow(t *testing.T) {
	source := "before\n| host | status |\n| --- | --- |\n| lab.internal | open |\n| second.internal | closed |\nafter"
	for _, width := range []int{12, 100} {
		rows, _ := Render(source, width, config.Default().Palette(), 0)
		for _, word := range []struct {
			text string
			line int
		}{{"open", 3}, {"closed", 4}, {"after", 5}} {
			found := false
			for _, row := range rows {
				if strings.Contains(ansi.Strip(row.Text), word.text) {
					found = true
					if row.Source != word.line {
						t.Fatalf("%s: source %d, want %d", word.text, row.Source, word.line)
					}
				}
			}
			if !found {
				t.Fatalf("missing %s", word.text)
			}
		}
	}
}
