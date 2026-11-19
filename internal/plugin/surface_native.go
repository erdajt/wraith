package plugin

import (
	"errors"
	"fmt"
	"slices"
)

// HostUpsert maintains two bounded native mirrors separately from plugin buffer capacity.
func (s *SurfaceStore) HostUpsert(slot string, lines []string, truncated bool) (SurfaceDelta, error) {
	if slot != "prompt" && slot != "conversation" {
		return SurfaceDelta{}, errors.New("native projection must be prompt or conversation")
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	size, err := validateSurfaceLines(lines, 4<<20)
	if err != nil {
		return SurfaceDelta{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return SurfaceDelta{}, errors.New("surface runtime is closed")
	}
	var current *surfaceBufferRecord
	for _, buffer := range s.buffers {
		if buffer.Projection == slot {
			current = buffer
			break
		}
	}
	if current == nil {
		s.serial++
		id := fmt.Sprintf("surface.%s.buffer.%d", s.prefix, s.serial)
		current = &surfaceBufferRecord{SurfaceBuffer: SurfaceBuffer{ID: id, Owner: "core", Name: slot, Projection: slot}}
		s.buffers[id] = current
	}
	if current.Revision == 0 || current.Truncated != truncated || !slices.Equal(current.Lines, lines) {
		if current.Revision == 0 || !slices.Equal(current.Lines, lines) {
			current.TextRevision++
		}
		current.Lines, current.bytes = slices.Clone(lines), size
		current.Truncated, current.Editable = truncated, slot == "prompt" && !truncated
		current.Spans, current.decorations = nil, nil
		current.Revision++
	}
	copy := cloneSurfaceBuffer(current.SurfaceBuffer)
	return SurfaceDelta{Operation: "buffer.native", Buffer: &copy, ChangedWindows: s.clampBufferWindows(current)}, nil
}
