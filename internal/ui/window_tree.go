package ui

type windowRect struct{ x, y, width, height int }
type windowNode struct {
	id       uint64
	vertical bool
	ratio    float64
	first    *windowNode
	second   *windowNode
}

type windowPlace struct {
	id uint64
	windowRect
}

// swap exchanges leaf positions without changing their editor state.
func (n *windowNode) swap(first, second uint64) {
	if n.id == first {
		n.id = second
	} else if n.id == second {
		n.id = first
	}
	if n.first != nil {
		n.first.swap(first, second)
		n.second.swap(first, second)
	}
}

// contains reports whether a leaf belongs to this subtree.
func (n *windowNode) contains(id uint64) bool {
	return n != nil && (n.id == id || n.first.contains(id) || n.second.contains(id))
}

// layout measures visible leaves, retaining the focused branch when space is limited.
func (n *windowNode) layout(r windowRect, focused uint64) []windowPlace {
	if n.first == nil {
		return []windowPlace{{n.id, r}}
	}
	size, minimum := r.height, 4
	if n.vertical {
		size, minimum = r.width, 24
	}
	if size < minimum*2+1 {
		branch := n.first
		if n.second.contains(focused) {
			branch = n.second
		}
		return branch.layout(r, focused)
	}

	cut := max(minimum, min(size-minimum-1, int(float64(size-1)*n.ratio)))
	a, b := r, r
	if n.vertical {
		a.width, b.x, b.width = cut, r.x+cut+1, size-cut-1
	} else {
		a.height, b.y, b.height = cut, r.y+cut+1, size-cut-1
	}
	return append(n.first.layout(a, focused), n.second.layout(b, focused)...)
}

// split replaces a leaf with two children while preserving the original leaf id.
func (n *windowNode) split(id, next uint64, vertical bool) bool {
	if n.id == id {
		*n = windowNode{vertical: vertical, ratio: 0.5, first: &windowNode{id: id}, second: &windowNode{id: next}}
		return true
	}
	return n.first != nil && (n.first.split(id, next, vertical) || n.second.split(id, next, vertical))
}

// remove collapses a leaf's parent into its surviving sibling.
func (n *windowNode) remove(id uint64) bool {
	if n.first == nil {
		return false
	}
	if n.first.id == id {
		*n = *n.second
		return true
	}
	if n.second.id == id {
		*n = *n.first
		return true
	}
	return n.first.remove(id) || n.second.remove(id)
}

// equalize restores equal sibling proportions throughout the tree.
func (n *windowNode) equalize() {
	if n.first != nil {
		n.ratio = 0.5
		n.first.equalize()
		n.second.equalize()
	}
}

// resize adjusts the nearest matching divider along a leaf's ancestry.
func (n *windowNode) resize(id uint64, vertical bool, delta float64) bool {
	if n.first == nil || !n.contains(id) {
		return false
	}
	if n.first.resize(id, vertical, delta) || n.second.resize(id, vertical, delta) {
		return true
	}
	if n.vertical != vertical {
		return false
	}
	if n.second.contains(id) {
		delta = -delta
	}
	n.ratio = max(0.1, min(0.9, n.ratio+delta))
	return true
}

// neighbor selects the closest leaf sharing the requested directional edge.
func neighbor(places []windowPlace, current uint64, direction string) uint64 {
	var origin windowRect
	for _, p := range places {
		if p.id == current {
			origin = p.windowRect
		}
	}
	chosen, distance := current, int(^uint(0)>>1)
	for _, p := range places {
		if p.id == current {
			continue
		}
		dx, dy := p.x-origin.x, p.y-origin.y
		eligible := false
		switch direction {
		case "h":
			eligible = p.x+p.width <= origin.x && p.y < origin.y+origin.height && p.y+p.height > origin.y
		case "l":
			eligible = p.x >= origin.x+origin.width && p.y < origin.y+origin.height && p.y+p.height > origin.y
		case "k":
			eligible = p.y+p.height <= origin.y && p.x < origin.x+origin.width && p.x+p.width > origin.x
		case "j":
			eligible = p.y >= origin.y+origin.height && p.x < origin.x+origin.width && p.x+p.width > origin.x
		}
		d := dx*dx + dy*dy
		if eligible && d < distance {
			chosen, distance = p.id, d
		}
	}
	return chosen
}

// setExtent sets the nearest matching divider from a requested leaf size in terminal cells.
func (n *windowNode) setExtent(id uint64, vertical bool, cells int, rect windowRect) bool {
	if n.first == nil || !n.contains(id) {
		return false
	}
	size, minimum := rect.height, 4
	if n.vertical {
		size, minimum = rect.width, 24
	}
	cut := max(minimum, min(size-minimum-1, int(float64(size-1)*n.ratio)))
	first, second := rect, rect
	if n.vertical {
		first.width, second.width = cut, size-cut-1
	} else {
		first.height, second.height = cut, size-cut-1
	}
	if n.first.setExtent(id, vertical, cells, first) || n.second.setExtent(id, vertical, cells, second) {
		return true
	}
	if n.vertical != vertical || size < minimum*2+1 {
		return false
	}
	if cells == 0 {
		n.ratio = 0.5
		return true
	}
	wanted := max(minimum, min(cells, size-minimum-1))
	if n.second.contains(id) {
		wanted = size - wanted - 1
	}
	n.ratio = float64(wanted) / float64(size-1)
	return true
}
