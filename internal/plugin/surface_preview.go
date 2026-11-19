package plugin

import (
	"maps"
	"slices"
)

// snapshot copies mutable store metadata while sharing immutable text between callback revisions.
func (s *SurfaceStore) snapshot() *SurfaceStore {
	s.mu.RLock()
	defer s.mu.RUnlock()
	copy := &SurfaceStore{
		prefix: s.prefix, serial: s.serial, closed: s.closed,
		owners: maps.Clone(s.owners), reserved: maps.Clone(s.reserved),
		buffers: make(map[string]*surfaceBufferRecord, len(s.buffers)),
		windows: maps.Clone(s.windows), regions: maps.Clone(s.regions),
	}
	for id, buffer := range s.buffers {
		record := *buffer
		record.decorations = slices.Clone(buffer.decorations)
		copy.buffers[id] = &record
	}
	return copy
}

// pendingReservations keeps only ids whose creation has not been committed or abandoned.
func (s *SurfaceStore) pendingReservations(ids []string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var pending []string
	for _, id := range ids {
		if _, exists := s.reserved[id]; exists {
			pending = append(pending, id)
		}
	}
	return pending
}
