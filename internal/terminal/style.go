package terminal

import (
	"fmt"
	"strconv"
)

func (s *Screen) sgr(p []int) {
	for i := 0; i < len(p); i++ {
		n := p[i]
		switch {
		case n == 0:
			s.style = Style{}
		case n == 1:
			s.style.Bold = true
		case n == 3:
			s.style.Italic = true
		case n == 4:
			s.style.Underline = true
		case n == 7:
			s.style.Inverse = true
		case n == 22:
			s.style.Bold = false
		case n == 23:
			s.style.Italic = false
		case n == 24:
			s.style.Underline = false
		case n == 27:
			s.style.Inverse = false
		case n == 39:
			s.style.FG = ""
		case n == 49:
			s.style.BG = ""
		case n >= 30 && n <= 37 || n >= 90 && n <= 97:
			s.style.FG = strconv.Itoa(n)
		case n >= 40 && n <= 47 || n >= 100 && n <= 107:
			s.style.BG = strconv.Itoa(n)
		case (n == 38 || n == 48) && i+2 < len(p):
			color := ""
			if p[i+1] == 5 {
				color = fmt.Sprintf("%d;5;%d", n, min(255, max(0, p[i+2])))
				i += 2
			} else if p[i+1] == 2 && i+4 < len(p) {
				color = fmt.Sprintf("%d;2;%d;%d;%d", n, min(255, max(0, p[i+2])), min(255, max(0, p[i+3])), min(255, max(0, p[i+4])))
				i += 4
			}
			if n == 38 {
				s.style.FG = color
			} else {
				s.style.BG = color
			}
		}
	}
}
