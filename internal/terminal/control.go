package terminal

import (
	"fmt"
	"strconv"
	"strings"
)

func (s *Screen) csi(raw string, final byte) []byte {
	private := strings.HasPrefix(raw, "?")
	raw = strings.TrimLeft(raw, "?>!")
	parts := strings.Split(raw, ";")
	p := make([]int, len(parts))
	for i, v := range parts {
		p[i], _ = strconv.Atoi(v)
	}
	arg := func(i, def int) int {
		if i >= len(p) || p[i] == 0 {
			return def
		}
		return min(p[i], 100000)
	}
	n := arg(0, 1)
	s.wrap = false
	switch final {
	case 'A':
		s.Y = max(0, s.Y-n)
	case 'B', 'e':
		s.Y = min(s.Height-1, s.Y+n)
	case 'C', 'a':
		s.X = min(s.Width-1, s.X+n)
	case 'D':
		s.X = max(0, s.X-n)
	case 'E':
		s.Y = min(s.Height-1, s.Y+n)
		s.X = 0
	case 'F':
		s.Y = max(0, s.Y-n)
		s.X = 0
	case 'G', '`':
		s.X = min(s.Width-1, n-1)
	case 'd':
		s.Y = min(s.Height-1, n-1)
	case 'H', 'f':
		s.Y = min(s.Height-1, n-1)
		s.X = min(s.Width-1, arg(1, 1)-1)
	case 'J':
		switch p[0] {
		case 0:
			s.clearRow(s.Y, s.X, s.Width)
			for y := s.Y + 1; y < s.Height; y++ {
				s.clearRow(y, 0, s.Width)
			}
		case 1:
			for y := 0; y < s.Y; y++ {
				s.clearRow(y, 0, s.Width)
			}
			s.clearRow(s.Y, 0, s.X+1)
		case 2:
			for y := range s.Cells {
				s.clearRow(y, 0, s.Width)
			}
		case 3:
			s.History = nil
			s.Offset = 0
		}
	case 'K':
		switch p[0] {
		case 0:
			s.clearRow(s.Y, s.X, s.Width)
		case 1:
			s.clearRow(s.Y, 0, s.X+1)
		case 2:
			s.clearRow(s.Y, 0, s.Width)
		}
	case 'X':
		s.clearRow(s.Y, s.X, s.X+n)
	case 'P':
		n = min(n, s.Width-s.X)
		copy(s.Cells[s.Y][s.X:], s.Cells[s.Y][s.X+n:])
		s.clearRow(s.Y, s.Width-n, s.Width)
	case '@':
		n = min(n, s.Width-s.X)
		copy(s.Cells[s.Y][s.X+n:], s.Cells[s.Y][s.X:s.Width-n])
		s.clearRow(s.Y, s.X, s.X+n)
	case 'L':
		if s.Y >= s.top && s.Y <= s.bottom {
			old := s.top
			s.top = s.Y
			s.scrollDown(n)
			s.top = old
		}
	case 'M':
		if s.Y >= s.top && s.Y <= s.bottom {
			old := s.top
			s.top = s.Y
			s.scrollUp(n)
			s.top = old
		}
	case 'S':
		s.scrollUp(n)
	case 'T':
		s.scrollDown(n)
	case 'r':
		top, bottom := n-1, arg(1, s.Height)-1
		if top < bottom && bottom < s.Height {
			s.top = top
			s.bottom = bottom
			s.X = 0
			s.Y = 0
		}
	case 's':
		s.savedX = s.X
		s.savedY = s.Y
	case 'u':
		s.X = min(s.savedX, s.Width-1)
		s.Y = min(s.savedY, s.Height-1)
	case 'm':
		s.sgr(p)
	case 'n':
		if p[0] == 6 {
			return fmt.Appendf(nil, "\x1b[%d;%dR", s.Y+1, s.X+1)
		}
		if p[0] == 5 {
			return []byte("\x1b[0n")
		}
	case 'c':
		return []byte("\x1b[?1;2c")
	case 'h', 'l':
		if private {
			for _, mode := range p {
				on := final == 'h'
				switch mode {
				case 25:
					s.Cursor = on
				case 2004:
					s.Bracketed = on
				case 1:
					s.Application = on
				case 1049, 1047, 47:
					if on && s.main == nil {
						s.main = s.Cells
						s.mainX = s.X
						s.mainY = s.Y
						s.Cells = nil
						s.Resize(s.Width, s.Height)
						s.X = 0
						s.Y = 0
						s.Offset = 0
					} else if !on && s.main != nil {
						s.Cells = s.main
						s.main = nil
						s.X = min(s.mainX, s.Width-1)
						s.Y = min(s.mainY, s.Height-1)
					}
				}
			}
		}
	}
	return nil
}
