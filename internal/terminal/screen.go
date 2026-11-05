package terminal

import "github.com/mattn/go-runewidth"

type Style struct {
	FG, BG                           string
	Bold, Italic, Underline, Inverse bool
}
type Cell struct {
	Text  string
	Width int
	Style Style
}
type Screen struct {
	Width, Height                  int
	X, Y                           int
	Cells                          [][]Cell
	History                        [][]Cell
	Limit, Offset                  int
	Cursor, Bracketed, Application bool
	style                          Style
	savedX, savedY                 int
	top, bottom                    int
	state                          byte
	seq                            []byte
	input                          []byte
	wrap                           bool
	main                           [][]Cell
	mainX, mainY                   int
}

func NewScreen(w, h, limit int) *Screen {
	s := &Screen{Limit: limit, Cursor: true}
	s.Resize(w, h)
	return s
}

func row(w int) []Cell { return make([]Cell, w) }

func (s *Screen) Resize(w, h int) {
	w = max(1, w)
	h = max(1, h)
	resize := func(old [][]Cell) [][]Cell {
		out := make([][]Cell, h)
		for y := range out {
			out[y] = row(w)
			if y < len(old) {
				copy(out[y], old[y])
				if out[y][w-1].Width == 2 {
					out[y][w-1] = Cell{}
				}
			}
		}
		return out
	}
	s.Cells = resize(s.Cells)
	if s.main != nil {
		s.main = resize(s.main)
	}
	s.Width = w
	s.Height = h
	s.X = min(s.X, w-1)
	s.Y = min(s.Y, h-1)
	s.top = 0
	s.bottom = h - 1
	s.wrap = false
}

func (s *Screen) put(r rune) {
	w := runewidth.RuneWidth(r)
	if w == 0 {
		x := s.X - 1
		if s.wrap {
			x = s.X
		}
		if x >= 0 {
			if s.Cells[s.Y][x].Width == -1 && x > 0 {
				x--
			}
			s.Cells[s.Y][x].Text += string(r)
		}
		return
	}
	if s.wrap || s.X+w > s.Width {
		s.X = 0
		s.linefeed()
		s.wrap = false
	}
	if w > s.Width {
		return
	}
	if s.X > 0 && s.Cells[s.Y][s.X].Width == -1 {
		s.Cells[s.Y][s.X-1] = Cell{}
	}
	if s.Cells[s.Y][s.X].Width == 2 && s.X+1 < s.Width {
		s.Cells[s.Y][s.X+1] = Cell{}
	}
	s.Cells[s.Y][s.X] = Cell{Text: string(r), Width: w, Style: s.style}
	if w == 2 {
		s.Cells[s.Y][s.X+1] = Cell{Width: -1, Style: s.style}
	}
	if s.X+w >= s.Width {
		s.X = s.Width - 1
		s.wrap = true
	} else {
		s.X += w
	}
}

func (s *Screen) linefeed() {
	s.wrap = false
	if s.Y == s.bottom {
		s.scrollUp(1)
	} else {
		s.Y = min(s.Height-1, s.Y+1)
	}
}
func (s *Screen) blank() Cell { return Cell{Style: s.style} }
func (s *Screen) clearRow(y, from, to int) {
	for x := max(0, from); x < min(to, s.Width); x++ {
		s.Cells[y][x] = s.blank()
	}
}

func (s *Screen) scrollUp(n int) {
	for range min(n, s.bottom-s.top+1) {
		if s.top == 0 && s.bottom == s.Height-1 && s.main == nil && s.Limit > 0 {
			s.History = append(s.History, append([]Cell(nil), s.Cells[0]...))
			if len(s.History) > s.Limit {
				s.History = s.History[len(s.History)-s.Limit:]
			}
		}
		copy(s.Cells[s.top:s.bottom], s.Cells[s.top+1:s.bottom+1])
		s.Cells[s.bottom] = row(s.Width)
	}
}
func (s *Screen) scrollDown(n int) {
	for range min(n, s.bottom-s.top+1) {
		copy(s.Cells[s.top+1:s.bottom+1], s.Cells[s.top:s.bottom])
		s.Cells[s.top] = row(s.Width)
	}
}
