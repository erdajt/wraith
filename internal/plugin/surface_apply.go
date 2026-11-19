package plugin

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Apply validates an explicit caller receipt before committing one ordered surface mutation.
func (s *SurfaceStore) Apply(mutation SurfaceMutation) (SurfaceDelta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || mutation.Lease == "" || s.owners[mutation.Owner] != mutation.Lease {
		return SurfaceDelta{}, errors.New("surface caller is no longer active")
	}
	return s.apply(mutation)
}

// HostApply commits UI-owned edits without a Lua receipt while preserving revision checks.
func (s *SurfaceStore) HostApply(mutation SurfaceMutation) (SurfaceDelta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return SurfaceDelta{}, errors.New("surface runtime is closed")
	}
	if mutation.Owner == "" {
		mutation.Owner = "core"
	}
	if s.owners[mutation.Owner] == "" {
		return SurfaceDelta{}, errors.New("surface owner is no longer active")
	}
	return s.apply(mutation)
}

// apply dispatches validated host mutations while the store holds its write lock.
func (s *SurfaceStore) apply(m SurfaceMutation) (SurfaceDelta, error) {
	if strings.HasPrefix(m.Operation, "region.") {
		return s.applyRegion(m)
	}
	if strings.HasPrefix(m.Operation, "window.") {
		return s.applyWindow(m)
	}
	if !strings.HasPrefix(m.Operation, "buffer.") {
		return SurfaceDelta{}, fmt.Errorf("unknown surface operation: %s", m.Operation)
	}
	current := s.buffers[m.ID]
	delta := SurfaceDelta{Operation: m.Operation}
	if m.Operation == "buffer.create" {
		reservation, ok := s.reserved[m.ID]
		if !ok || reservation.kind != "buffer" || m.Expected != 0 || current != nil || m.Buffer == nil {
			return delta, errors.New("buffer creation requires a live reservation and revision zero")
		}
		buffer := cloneSurfaceBuffer(*m.Buffer)
		if len(buffer.Keymaps) > 0 {
			return delta, errors.New("create buffer mappings with set_keymap")
		}
		if buffer.Projection != "" || buffer.Truncated {
			return delta, errors.New("native projections can only be created by the host")
		}
		buffer.ID, buffer.Owner, buffer.Revision = m.ID, reservation.owner, 1
		if len(buffer.Lines) == 0 {
			buffer.Lines = []string{""}
		}
		if len(buffer.Name) > 256 {
			return delta, errors.New("buffer name exceeds 256 bytes")
		}
		if _, err := validateSurfaceLines([]string{buffer.Name}, 257); err != nil {
			return delta, err
		}
		size, err := validateSurfaceLines(buffer.Lines, 4<<20)
		if err != nil {
			return delta, err
		}
		if err := validateSurfaceSpans(buffer.Lines, buffer.Spans); err != nil {
			return delta, err
		}
		total := size
		for _, existing := range s.buffers {
			if existing.Projection == "" {
				total += existing.bytes
			}
		}
		if total > 16<<20 {
			return delta, errors.New("buffer store exceeds 16 mib")
		}
		record := &surfaceBufferRecord{SurfaceBuffer: buffer, bytes: size}
		if len(buffer.Spans) > 0 {
			record.decorations = []surfaceDecoration{{owner: m.Owner, spans: slices.Clone(buffer.Spans)}}
		}
		s.buffers[m.ID] = record
		delete(s.reserved, m.ID)
		copy := cloneSurfaceBuffer(buffer)
		delta.Buffer = &copy
		return delta, nil
	}
	if current == nil {
		return delta, fmt.Errorf("buffer is no longer available: %s", m.ID)
	}
	if m.Expected != current.Revision {
		return delta, fmt.Errorf("buffer %s changed: expected revision %d, current %d", m.ID, m.Expected, current.Revision)
	}
	switch m.Operation {
	case "buffer.set_keymap", "buffer.del_keymap":
		if err := setBufferKeymap(current, m); err != nil {
			return delta, err
		}
	case "buffer.delete":
		if current.Projection != "" {
			return delta, errors.New("native projections cannot be deleted")
		}
		delete(s.buffers, m.ID)
		delta.RemovedBuffers = []string{m.ID}
		for id, window := range s.windows {
			if window.BufferID == m.ID {
				delete(s.windows, id)
				delta.RemovedWindows = append(delta.RemovedWindows, id)
			}
		}
		slices.Sort(delta.RemovedWindows)
		return delta, nil
	case "buffer.set_lines":
		if current.Projection == "conversation" || current.Projection != "" && current.Truncated {
			return delta, errors.New("conversation and incomplete prompt projections are read-only")
		}
		if m.Start < 0 || m.End < m.Start || m.End > len(current.Lines) {
			return delta, errors.New("invalid buffer line replacement range")
		}
		lines := make([]string, 0, len(current.Lines)-(m.End-m.Start)+len(m.Lines))
		lines = append(lines, current.Lines[:m.Start]...)
		lines = append(lines, m.Lines...)
		lines = append(lines, current.Lines[m.End:]...)
		if len(lines) == 0 {
			lines = []string{""}
		}
		size, err := validateSurfaceLines(lines, 4<<20)
		if err != nil {
			return delta, err
		}
		total := 0
		if current.Projection == "" {
			total = size - current.bytes
		}
		for _, buffer := range s.buffers {
			if buffer.Projection == "" {
				total += buffer.bytes
			}
		}
		if total > 16<<20 {
			return delta, errors.New("buffer store exceeds 16 mib")
		}
		if !slices.Equal(lines, current.Lines) {
			current.Lines, current.bytes = lines, size
			current.Spans, current.decorations = nil, nil
			current.Revision++
			delta.ChangedWindows = s.clampBufferWindows(current)
		}
	case "buffer.set_spans":
		if err := validateSurfaceSpans(current.Lines, m.Spans); err != nil {
			return delta, err
		}
		layers := slices.DeleteFunc(slices.Clone(current.decorations), func(layer surfaceDecoration) bool { return layer.owner == m.Owner })
		if len(m.Spans) > 0 {
			layers = append(layers, surfaceDecoration{owner: m.Owner, spans: slices.Clone(m.Spans)})
		}
		var spans []SurfaceSpan
		unchanged := len(layers) == len(current.decorations)
		for i, layer := range layers {
			spans = append(spans, layer.spans...)
			unchanged = unchanged && layer.owner == current.decorations[i].owner && slices.Equal(layer.spans, current.decorations[i].spans)
		}
		if len(spans) > 4096 {
			return delta, errors.New("buffer decorations exceed 4096 spans")
		}
		if !unchanged {
			current.decorations, current.Spans = layers, spans
			current.Revision++
		}
	default:
		return delta, fmt.Errorf("unknown surface operation: %s", m.Operation)
	}
	copy := cloneSurfaceBuffer(current.SurfaceBuffer)
	delta.Buffer = &copy
	return delta, nil
}
