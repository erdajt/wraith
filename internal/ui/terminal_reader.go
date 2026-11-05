package ui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/terminal"
)

func terminalText(s *terminal.Screen) string {
	var lines []string
	for _, rows := range [][][]terminal.Cell{s.History, s.Cells} {
		for _, row := range rows {
			var line strings.Builder
			for _, cell := range row {
				if cell.Width < 0 {
					continue
				}
				if cell.Text == "" {
					line.WriteByte(' ')
				} else {
					line.WriteString(cell.Text)
				}
			}
			lines = append(lines, strings.TrimRight(line.String(), " "))
		}
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

func (a *App) terminalReaderKey(index int, k tea.KeyMsg) (bool, tea.Cmd) {
	p := a.panes[index]
	if p.Reader != nil {
		b := p.Reader.Buffer
		if k.String() == a.Config.Keys["output_share"] && !a.Config.Execution.AutoShare {
			return true, a.shareTerminal(index)
		}
		if k.String() == "r" && b.Pending == "" && b.Mode == "normal" {
			p.Reader = newReader(terminalText(p.Screen))
			return true, nil
		}
		_, h := a.paneSize(index)
		text, exit := p.Reader.key(k, h-3)
		if exit {
			p.Reader = nil
		}
		if text != "" {
			return true, a.copyText(text, b.Linewise)
		}
		return true, nil
	}
	if k.Type == tea.KeyEsc && p.Pending == "" {
		if time.Since(p.EscapedAt) > time.Second {
			p.Escapes = 0
		}
		p.EscapedAt = time.Now()
		p.Escapes++
		if k.Alt {
			p.Escapes++
		}
		if p.Escapes >= 3 {
			p.Escapes = 0
			p.Reader = newReader(terminalText(p.Screen))
			p.Reader.Buffer.Key("G", "G")
			return true, nil
		}
	} else {
		p.Escapes = 0
	}
	return false, nil
}
