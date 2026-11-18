package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestWelcomeKeepsItsSilhouetteAndLayoutAcrossFrames(t *testing.T) {
	a := fixture(t)
	p := a.Config.Palette()

	for _, size := range [][2]int{{30, 5}, {40, 10}, {50, 14}, {80, 18}, {80, 20}, {110, 24}} {
		var previous []string
		var silhouette string
		changed := false
		for frame := range 36 {
			a.frame = frame
			rows := a.welcome(size[0], size[1], p)
			if len(rows) > size[1] {
				t.Fatalf("welcome exceeds available height %v", size)
			}

			for i, row := range rows {
				if ansi.StringWidth(row) > size[0] {
					t.Fatalf("welcome exceeds available width %v", size)
				}

				if previous != nil && ansi.StringWidth(row) != ansi.StringWidth(previous[i]) {
					t.Fatal("animation shifts the centered artwork")
				}
			}

			plain := ansi.Strip(strings.Join(rows, "\n"))
			if frame == 0 {
				silhouette = plain
			} else if plain != silhouette {
				t.Fatal("animation changes the mark instead of its lighting")
			}

			if previous != nil && strings.Join(rows, "\n") != strings.Join(previous, "\n") {
				changed = true
			}
			previous = rows
		}

		if size[1] >= 10 && !changed {
			t.Fatal("specter does not animate")
		}
	}
}

func TestDisabledWelcomeAnimationKeepsAStableFrame(t *testing.T) {
	a := fixture(t)
	a.Config.Animations = false
	a.frame = 1
	first := strings.Join(a.welcome(100, 24, a.Config.Palette()), "\n")
	a.frame = 19

	if first != strings.Join(a.welcome(100, 24, a.Config.Palette()), "\n") {
		t.Fatal("disabled animation kept moving")
	}
}

func TestWelcomeHoodStaysCenteredAtCompactThreshold(t *testing.T) {
	p := fixture(t).Config.Palette()

	for _, height := range []int{10, 14, 17, 18, 24} {
		for frame := range 36 {
			rows := wraithEmblem(height, frame, p)
			width := ansi.StringWidth(rows[0])

			for y, styled := range rows {
				row := []rune(ansi.Strip(styled))
				if len(row) != width {
					t.Fatalf("height %d row %d changes grid width", height, y)
				}

				for x, cell := range row {
					mirror := row[len(row)-1-x]
					switch mirror {
					case '▐':
						mirror = '▌'
					case '▌':
						mirror = '▐'
					}

					if cell != mirror {
						t.Fatalf("height %d row %d column %d breaks hood symmetry", height, y, x)
					}
				}
			}
		}
	}
}
