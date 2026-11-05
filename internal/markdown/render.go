package markdown

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/erdajt/wraith/internal/config"
)

var heading = regexp.MustCompile(`^ {0,3}#{1,6}\s+(.+?)\s*#*\s*$`)
var list = regexp.MustCompile(`^(\s*)(?:[-+*]|([0-9]+)[.)])\s+(.*)$`)
var rule = regexp.MustCompile(`^ {0,3}(?:\*\s*){3,}$|^ {0,3}(?:-\s*){3,}$|^ {0,3}(?:_\s*){3,}$`)

type Row struct {
	Text  string
	Block int
}

func Render(source string, width int, p config.Palette, block int) ([]Row, int) {
	width = max(4, width)
	var out []Row
	active, localBlock := -1, 0
	add := func(s string) {
		for _, line := range strings.Split(ansi.Hardwrap(s, width, true), "\n") {
			out = append(out, Row{Text: line, Block: active})
		}
	}
	lines := strings.Split(source, "\n")
	fence := ""
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trim := strings.TrimSpace(line)
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if fence != "" {
			if indent <= 3 && strings.HasPrefix(trim, fence) && strings.Trim(trim, string(fence[0])+" \t") == "" {
				add(paint(p.Border, "╰"+strings.Repeat("─", min(width-1, 40))))
				fence = ""
				active = -1
			} else {
				add(paint(p.Border, "│ ") + paint(p.Accent, line))
			}
			continue
		}
		if indent <= 3 && (strings.HasPrefix(trim, "```") || strings.HasPrefix(trim, "~~~")) {
			n := 0
			for n < len(trim) && trim[n] == trim[0] {
				n++
			}
			fence = trim[:n]
			block++
			active = localBlock
			localBlock++
			add(paint(p.Border, "╭─ ") + paint(p.Second, fmt.Sprintf("%s · block %d", strings.TrimSpace(trim[n:]), block)) + paint(p.Muted, " · /blocks"))
			continue
		}

		if i+1 < len(lines) && tableSeparator(lines[i+1]) && strings.Contains(line, "|") {
			table, consumed := renderTable(lines[i:], width, p)
			for _, row := range table {
				add(row)
			}
			i += consumed - 1
			continue
		}
		if m := heading.FindStringSubmatch(line); m != nil {
			add(inline(m[1], p, ";1", 0))
			continue
		}
		if rule.MatchString(line) {
			add(paint(p.Border, strings.Repeat("─", min(width, 40))))
			continue
		}
		if strings.HasPrefix(trim, ">") {
			add(paint(p.Border, "│ ") + inline(strings.TrimSpace(strings.TrimPrefix(trim, ">")), p, "", 0))
			continue
		}
		if m := list.FindStringSubmatch(line); m != nil {
			marker := "• "
			if m[2] != "" {
				marker = m[2] + ". "
			}
			text := m[3]
			if strings.HasPrefix(text, "[ ] ") {
				marker = "☐ "
				text = text[4:]
			}
			if strings.HasPrefix(text, "[x] ") || strings.HasPrefix(text, "[X] ") {
				marker = "☑ "
				text = text[4:]
			}
			add(m[1] + paint(p.Second, marker) + inline(text, p, "", 0))
			continue
		}
		add(inline(line, p, "", 0))
	}
	return out, block
}

func tableSeparator(line string) bool {
	cells := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
	if len(cells) < 2 {
		return false
	}
	for _, cell := range cells {
		cell = strings.Trim(strings.TrimSpace(cell), ":")
		if len(cell) < 3 || strings.Trim(cell, "-") != "" {
			return false
		}
	}
	return true
}
