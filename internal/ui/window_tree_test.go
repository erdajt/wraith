package ui

import "testing"

func TestWindowTreeRetainsFocusedLeafAndRestoresLayout(t *testing.T) {
	n := &windowNode{id: 1}
	n.split(1, 2, true)
	n.split(2, 3, false)
	full := windowRect{2, 3, 101, 21}
	places := n.layout(full, 3)
	if len(places) != 3 || places[0].width != 50 || places[1].height != 10 || places[2].y != 14 {
		t.Fatalf("nested geometry: %+v", places)
	}
	for _, direction := range []struct {
		from, want uint64
		key        string
	}{{1, 2, "l"}, {3, 2, "k"}, {2, 3, "j"}, {3, 1, "h"}} {
		if got := neighbor(places, direction.from, direction.key); got != direction.want {
			t.Errorf("%d %s: got %d, want %d", direction.from, direction.key, got, direction.want)
		}
	}
	shrunk := n.layout(windowRect{0, 0, 40, 7}, 3)
	if len(shrunk) != 1 || shrunk[0].id != 3 {
		t.Fatalf("focused leaf lost: %+v", shrunk)
	}
	restored := n.layout(full, 3)
	for i := range places {
		if places[i] != restored[i] {
			t.Fatal("resize changed stored layout")
		}
	}
}

func TestWindowTreeResizeAndClosePreserveSiblingIDs(t *testing.T) {
	n := &windowNode{id: 1}
	n.split(1, 2, true)
	n.split(2, 3, false)
	n.resize(3, true, .1)
	n.resize(3, false, .1)
	places := n.layout(windowRect{0, 0, 101, 21}, 3)
	if places[0].width != 40 || places[1].height != 8 {
		t.Fatalf("resize changed wrong divider: %+v", places)
	}
	n.equalize()
	if got := n.layout(windowRect{0, 0, 101, 21}, 3); got[0].width != 50 || got[1].height != 10 {
		t.Fatalf("equalize: %+v", got)
	}
	if !n.remove(2) || n.contains(2) || !n.contains(1) || !n.contains(3) {
		t.Fatal("closing nested leaf lost its sibling")
	}
	if !n.remove(1) || n.id != 3 || n.remove(3) {
		t.Fatal("last leaf must remain open")
	}
}

func TestWindowTreeVisibleRectanglesStayInsideAvailableSpace(t *testing.T) {
	n := &windowNode{id: 1}
	for id := uint64(1); id < 8; id++ {
		n.split(id, id+1, id%2 == 0)
	}
	for width := 20; width < 200; width += 7 {
		for height := 3; height < 60; height += 3 {
			places := n.layout(windowRect{0, 0, width, height}, 8)
			focused := false
			for i, p := range places {
				focused = focused || p.id == 8
				if p.x < 0 || p.y < 0 || p.x+p.width > width || p.y+p.height > height {
					t.Fatalf("outside %dx%d: %+v", width, height, p)
				}
				for _, other := range places[:i] {
					if p.x < other.x+other.width && p.x+p.width > other.x && p.y < other.y+other.height && p.y+p.height > other.y {
						t.Fatal("leaves overlap")
					}
				}
			}
			if !focused {
				t.Fatal("focused leaf hidden")
			}
		}
	}
}
