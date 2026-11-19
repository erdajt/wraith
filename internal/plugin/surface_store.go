package plugin

import (
	"errors"
	"fmt"
	"slices"
	"sync"
)

type surfaceReservation struct{ owner, kind string }
type surfaceDecoration struct {
	owner string
	spans []SurfaceSpan
}
type surfaceBinding struct {
	ownedBinding
	targetOwner string
}

type surfaceBufferRecord struct {
	SurfaceBuffer
	bytes       int
	decorations []surfaceDecoration
	bindings    map[string][]surfaceBinding
}

// SurfaceStore owns committed display data without holding Lua values or running callbacks.
type SurfaceStore struct {
	mu       sync.RWMutex
	prefix   string
	serial   uint64
	closed   bool
	owners   map[string]string
	reserved map[string]surfaceReservation
	buffers  map[string]*surfaceBufferRecord
	windows  map[string]SurfaceWindow
	regions  map[string]SurfaceRegion
}

// newSurfaceStore separates persistent display resources from managed worker capacity.
func newSurfaceStore(prefix string) *SurfaceStore {
	s := &SurfaceStore{
		prefix: prefix, owners: map[string]string{},
		reserved: map[string]surfaceReservation{}, buffers: map[string]*surfaceBufferRecord{},
		windows: map[string]SurfaceWindow{}, regions: map[string]SurfaceRegion{},
	}
	_, _ = s.registerOwner("config")
	_, _ = s.registerOwner("core")
	return s
}

// registerOwner issues a fresh lease after the VM owner verifies the package is still loaded.
func (s *SurfaceStore) registerOwner(owner string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || owner == "" {
		return "", errors.New("surface owner is no longer active")
	}
	if lease := s.owners[owner]; lease != "" {
		return lease, nil
	}
	s.serial++
	lease := fmt.Sprintf("surface.%s.owner.%d", s.prefix, s.serial)
	s.owners[owner] = lease
	return lease, nil
}

// OwnerLease returns the current owner receipt without creating or reviving a package.
func (s *SurfaceStore) OwnerLease(owner string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.owners[owner]
}

// Reserve allocates an invisible object id whose lifetime belongs to its creator.
func (s *SurfaceStore) Reserve(owner, kind string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.owners[owner] == "" {
		return "", errors.New("surface owner is no longer active")
	}
	if kind != "buffer" && kind != "window" && kind != "region" {
		return "", errors.New("unknown surface kind")
	}
	count := 0
	for _, buffer := range s.buffers {
		if buffer.Projection == "" {
			count++
		}
	}
	limit := 64
	if kind == "window" {
		count = len(s.windows)
	} else if kind == "region" {
		count = len(s.regions)
		limit = 32
	}
	for _, reservation := range s.reserved {
		if reservation.kind == kind {
			count++
		}
	}
	if count >= limit {
		return "", fmt.Errorf("%s limit reached; dispose unused surfaces", kind)
	}
	s.serial++
	id := fmt.Sprintf("surface.%s.%s.%d", s.prefix, kind, s.serial)
	s.reserved[id] = surfaceReservation{owner, kind}
	return id, nil
}

// Valid checks the caller lease independently of prompt focus or workspace snapshots.
func (s *SurfaceStore) Valid(mutation SurfaceMutation) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return !s.closed && mutation.Lease != "" && s.owners[mutation.Owner] == mutation.Lease
}

// DiscardReservations removes abandoned creations without deleting committed resources.
func (s *SurfaceStore) DiscardReservations(ids []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range ids {
		delete(s.reserved, id)
	}
}

// RevokeOwner disposes creator-owned surfaces and removes that caller's decorations elsewhere.
func (s *SurfaceStore) RevokeOwner(owner string) SurfaceDelta {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.owners, owner)
	delta := SurfaceDelta{Operation: "owner.delete"}
	for id, reservation := range s.reserved {
		if reservation.owner == owner {
			delete(s.reserved, id)
		}
	}
	for id, buffer := range s.buffers {
		if buffer.Owner == owner {
			delete(s.buffers, id)
			delta.RemovedBuffers = append(delta.RemovedBuffers, id)
			continue
		}
		bindingsChanged := false
		for key, layers := range buffer.bindings {
			kept := slices.DeleteFunc(layers, func(layer surfaceBinding) bool { return layer.owner == owner || layer.targetOwner == owner })
			bindingsChanged = bindingsChanged || len(kept) != len(layers)
			if len(kept) == 0 {
				delete(buffer.bindings, key)
			} else {
				buffer.bindings[key] = kept
			}
		}
		if bindingsChanged {
			buffer.refreshKeymaps()
		}
		previous := len(buffer.decorations)
		buffer.decorations = slices.DeleteFunc(buffer.decorations, func(layer surfaceDecoration) bool { return layer.owner == owner })
		if len(buffer.decorations) != previous || bindingsChanged {
			buffer.Spans = nil
			for _, layer := range buffer.decorations {
				buffer.Spans = append(buffer.Spans, layer.spans...)
			}
			buffer.Revision++
			delta.ChangedBuffers = append(delta.ChangedBuffers, cloneSurfaceBuffer(buffer.SurfaceBuffer))
		}
	}
	for id, window := range s.windows {
		if window.Owner == owner || slices.Contains(delta.RemovedBuffers, window.BufferID) {
			delete(s.windows, id)
			delta.RemovedWindows = append(delta.RemovedWindows, id)
		}
	}
	for id, region := range s.regions {
		if region.Owner == owner {
			delete(s.regions, id)
			delta.RemovedRegions = append(delta.RemovedRegions, id)
		}
	}
	slices.Sort(delta.RemovedBuffers)
	slices.Sort(delta.RemovedWindows)
	slices.Sort(delta.RemovedRegions)
	return delta
}

// close invalidates all receipts before the runtime begins arbitrary Lua teardown.
func (s *SurfaceStore) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	clear(s.owners)
	clear(s.reserved)
	clear(s.buffers)
	clear(s.windows)
	clear(s.regions)
}

// OwnerValid checks a package receipt for non-buffer UI requests such as dialogs.
func (s *SurfaceStore) OwnerValid(owner, lease string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return !s.closed && lease != "" && s.owners[owner] == lease
}

// cloneSurfaceBuffer detaches mutable slices while retaining immutable line strings.
func cloneSurfaceBuffer(buffer SurfaceBuffer) SurfaceBuffer {
	buffer.Lines = slices.Clone(buffer.Lines)
	buffer.Spans = slices.Clone(buffer.Spans)
	buffer.Keymaps = slices.Clone(buffer.Keymaps)
	return buffer
}
