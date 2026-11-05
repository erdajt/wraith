package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestWelcomeKeepsItsSilhouetteAndLayoutAcrossFrames(t *testing.T) {
	a := fixture(t)
	p := a.Config.Palette()
	for _, size := range [][2]int{{30, 5}, {40, 9}, {50, 14}, {80, 18}, {80, 20}, {110, 24}} {
		var previous []string
		changed := false
		for frame := range 24 {
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
			if previous != nil && strings.Join(rows, "\n") != strings.Join(previous, "\n") {
				changed = true
			}
			previous = rows
		}
		if size[1] >= 9 && !changed {
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
