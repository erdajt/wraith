package ui

import (
	"strings"
	"testing"
)

func TestWelcomeAnimationChangesFramesWithinBounds(t *testing.T) {
	one, two := wraithOrbit(36, 8, 1), wraithOrbit(36, 8, 10)
	if strings.Join(one, "\n") == strings.Join(two, "\n") {
		t.Fatal("orbit did not rotate")
	}
	for _, line := range append(one, two...) {
		if len(line) != 36 {
			t.Fatal("animation exceeded its bounds")
		}
	}
	a := fixture(t)
	a.Config.Animations = false
	a.frame = 1
	first := strings.Join(a.welcome(100, 20, a.Config.Palette()), "\n")
	a.frame = 10
	if first != strings.Join(a.welcome(100, 20, a.Config.Palette()), "\n") {
		t.Fatal("disabled animation kept moving")
	}
}
