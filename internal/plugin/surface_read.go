package plugin

import (
	"fmt"
	"slices"
	"strings"
)

// Buffer copies canonical text for the UI cache without exposing mutable store slices.
func (s *SurfaceStore) Buffer(id string) (SurfaceBuffer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	buffer := s.buffers[id]
	if s.closed || buffer == nil {
		return SurfaceBuffer{}, fmt.Errorf("buffer is no longer available: %s", id)
	}
	return cloneSurfaceBuffer(buffer.SurfaceBuffer), nil
}

// BufferLines returns at most one mib of complete lines from a zero-based half-open range.
func (s *SurfaceStore) BufferLines(id string, start, end int) (SurfaceBufferPage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	buffer := s.buffers[id]
	if s.closed || buffer == nil {
		return SurfaceBufferPage{}, fmt.Errorf("buffer is no longer available: %s", id)
	}
	if end == -1 {
		end = len(buffer.Lines)
	}
	if start < 0 || end < start || end > len(buffer.Lines) {
		return SurfaceBufferPage{}, fmt.Errorf("invalid buffer line range [%d,%d)", start, end)
	}
	next, size := start, 0
	for next < end {
		bytes := len(buffer.Lines[next]) + 1
		if size+bytes > 1<<20 {
			break
		}
		size += bytes
		next++
	}
	return SurfaceBufferPage{SurfaceBufferInfo: bufferInfo(buffer), Start: start, Lines: slices.Clone(buffer.Lines[start:next]), Next: next}, nil
}

// Window returns one independent view's committed layout and cursor metadata.
func (s *SurfaceStore) Window(id string) (SurfaceWindow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	window, ok := s.windows[id]
	if s.closed || !ok {
		return SurfaceWindow{}, fmt.Errorf("window is no longer available: %s", id)
	}
	return window, nil
}

// Region copies a complete bounded display frame for the UI cache.
func (s *SurfaceStore) Region(id string) (SurfaceRegion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	region, ok := s.regions[id]
	if s.closed || !ok {
		return SurfaceRegion{}, fmt.Errorf("region is no longer available: %s", id)
	}
	region.Lines = slices.Clone(region.Lines)
	region.Spans = slices.Clone(region.Spans)
	return region, nil
}

// Index lists deterministic metadata without copying text frames or invoking Lua.
func (s *SurfaceStore) Index() SurfaceIndex {
	s.mu.RLock()
	defer s.mu.RUnlock()
	index := SurfaceIndex{}
	for _, buffer := range s.buffers {
		index.Buffers = append(index.Buffers, bufferInfo(buffer))
	}
	for _, window := range s.windows {
		index.Windows = append(index.Windows, window)
	}
	for _, region := range s.regions {
		index.Regions = append(index.Regions, SurfaceRegionInfo{
			ID: region.ID, Owner: region.Owner, Slot: region.Slot, Revision: region.Revision, Order: region.Order,
			Width: region.Width, Height: region.Height, Row: region.Row, Column: region.Column, LineCount: len(region.Lines),
		})
	}
	slices.SortFunc(index.Buffers, func(a, b SurfaceBufferInfo) int { return strings.Compare(a.ID, b.ID) })
	slices.SortFunc(index.Windows, func(a, b SurfaceWindow) int { return strings.Compare(a.ID, b.ID) })
	slices.SortFunc(index.Regions, func(a, b SurfaceRegionInfo) int {
		if a.Order < b.Order {
			return -1
		}
		if a.Order > b.Order {
			return 1
		}
		return strings.Compare(a.ID, b.ID)
	})
	return index
}

// bufferInfo derives bounded metadata from the canonical record's cached byte count.
func bufferInfo(buffer *surfaceBufferRecord) SurfaceBufferInfo {
	return SurfaceBufferInfo{
		Projection: buffer.Projection, Truncated: buffer.Truncated,
		ID: buffer.ID, Owner: buffer.Owner, Name: buffer.Name, Revision: buffer.Revision, TextRevision: buffer.TextRevision, Editable: buffer.Editable,
		LineCount: len(buffer.Lines), ByteCount: buffer.bytes,
	}
}
