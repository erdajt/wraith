package terminal

import "unicode/utf8"

func (s *Screen) Feed(data []byte) []byte {
	s.input = append(s.input, data...)
	var reply []byte
	for len(s.input) > 0 {
		b := s.input[0]
		if s.state == 0 && b >= 0x80 {
			if !utf8.FullRune(s.input) {
				break
			}
			r, n := utf8.DecodeRune(s.input)
			s.input = s.input[n:]
			s.put(r)
			continue
		}
		s.input = s.input[1:]
		switch s.state {
		case 0:
			switch b {
			case 0x1b:
				s.state = 1
			case '\r':
				s.X = 0
				s.wrap = false
			case '\n', '\v', '\f':
				s.linefeed()
			case '\b':
				s.X = max(0, s.X-1)
				s.wrap = false
			case '\t':
				s.X = min(s.Width-1, (s.X/8+1)*8)
			default:
				if b >= 32 && b != 127 {
					s.put(rune(b))
				}
			}
		case 1:
			s.state = 0
			switch b {
			case '[':
				s.state = 2
				s.seq = nil
			case ']', 'P', '_', '^':
				s.state = 3
				s.seq = nil
			case '(', ')':
				s.state = 5
			case '7':
				s.savedX = s.X
				s.savedY = s.Y
			case '8':
				s.X = min(s.savedX, s.Width-1)
				s.Y = min(s.savedY, s.Height-1)
			case 'D':
				s.linefeed()
			case 'E':
				s.X = 0
				s.linefeed()
			case 'M':
				if s.Y == s.top {
					s.scrollDown(1)
				} else {
					s.Y = max(0, s.Y-1)
				}
			case 'c':
				s.Cells = nil
				s.Resize(s.Width, s.Height)
				s.X = 0
				s.Y = 0
				s.style = Style{}
			}
		case 2:
			if b >= 0x40 && b <= 0x7e {

				s.state = 0
				s.seq = nil
			} else if len(s.seq) < 4096 {
				s.seq = append(s.seq, b)
			} else {
				s.state = 0
				s.seq = nil
			}
		case 3:
			if b == 7 {
				s.state = 0
			} else if b == 0x1b {
				s.state = 4
			}
		case 4:
			if b == '\\' {
				s.state = 0
			} else {
				s.state = 3
			}
		case 5:
			s.state = 0
		}
	}
	return reply
}
