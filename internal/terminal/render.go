package terminal

import "strings"

func (st Style) ansi(fg, bg string) string {
	p := []string{"0"}
	if st.FG != "" {
		p = append(p, st.FG)
	} else if fg != "" {
		p = append(p, fg)
	}
	if st.BG != "" {
		p = append(p, st.BG)
	} else if bg != "" {
		p = append(p, bg)
	}
	if st.Bold {
		p = append(p, "1")
	}
	if st.Italic {
		p = append(p, "3")
	}
	if st.Underline {
		p = append(p, "4")
	}
	if st.Inverse {
		p = append(p, "7")
	}
	return "\x1b[" + strings.Join(p, ";") + "m"
}

func (s *Screen) Lines(focused bool, fg, bg string) []string {
	out := make([]string, s.Height)
	offset := min(s.Offset, len(s.History))
	start := len(s.History) - offset
	for y := range out {
		var cells []Cell
		idx := start + y
		if idx < len(s.History) {
			cells = s.History[idx]
		} else if idx-len(s.History) < len(s.Cells) {
			cells = s.Cells[idx-len(s.History)]
		}
		var b strings.Builder
		last := ""
		for x := 0; x < s.Width; x++ {
			var c Cell
			if x < len(cells) {
				c = cells[x]
			}
			if c.Width == -1 {
				continue
			}
			if focused && offset == 0 && s.Cursor && x == s.X && y == s.Y {
				c.Style.Inverse = !c.Style.Inverse
			}
			style := c.Style.ansi(fg, bg)
			if style != last {
				b.WriteString(style)
				last = style
			}
			if c.Text == "" {
				b.WriteByte(' ')
			} else {
				b.WriteString(c.Text)
			}
		}
		b.WriteString("\x1b[0m")
		out[y] = b.String()
	}
	return out
}

func (s *Screen) Scroll(n int) { s.Offset = min(len(s.History), max(0, s.Offset+n)) }
