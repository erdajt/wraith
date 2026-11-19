package plugin

import (
	"errors"
	"fmt"
	"slices"
)

// applyRegion validates cached frames without invoking Lua during rendering.
func (s *SurfaceStore) applyRegion(m SurfaceMutation) (SurfaceDelta, error) {
	delta := SurfaceDelta{Operation: m.Operation}
	current, exists := s.regions[m.ID]
	if m.Operation != "region.create" {
		if !exists {
			return delta, fmt.Errorf("region is no longer available: %s", m.ID)
		}
		if m.Expected != current.Revision {
			return delta, fmt.Errorf("region %s changed: expected revision %d, current %d", m.ID, m.Expected, current.Revision)
		}
	}
	if m.Operation == "region.delete" {
		delete(s.regions, m.ID)
		delta.RemovedRegions = []string{m.ID}
		return delta, nil
	}
	if m.Region == nil || m.Operation != "region.create" && m.Operation != "region.set" {
		return delta, errors.New("region mutation requires a frame")
	}
	region := *m.Region
	if m.Operation == "region.create" {
		reservation, ok := s.reserved[m.ID]
		if !ok || reservation.kind != "region" || m.Expected != 0 || exists {
			return delta, errors.New("region creation requires a live reservation and revision zero")
		}
		region.ID, region.Owner, region.Revision = m.ID, reservation.owner, 1
	} else {
		region.ID, region.Owner, region.Revision, region.Order = current.ID, current.Owner, current.Revision, current.Order
	}
	if !slices.Contains([]string{"header", "footer", "sidebar", "body", "welcome", "prompt"}, region.Slot) {
		return delta, errors.New("region slot must be header, footer, sidebar, body, welcome or prompt")
	}
	if region.Slot == "prompt" && len(region.Lines) > 4 || len(region.Lines) > 200 || (region.Slot == "header" || region.Slot == "footer") && len(region.Lines) > 1 {
		return delta, errors.New("region exceeds its slot line limit")
	}
	if _, err := validateSurfaceLines(region.Lines, 256<<10); err != nil {
		return delta, err
	}
	if err := validateSurfaceSpans(region.Lines, region.Spans); err != nil {
		return delta, err
	}
	for _, dimension := range []int{region.Width, region.Height, region.Row, region.Column} {
		if dimension < 0 || dimension > 10000 {
			return delta, errors.New("region dimensions must be between zero and 10000")
		}
	}
	if region.Slot == "sidebar" && (region.Width == 0 || region.Width > 120) {
		return delta, errors.New("sidebar region width must be between 1 and 120")
	}
	if region.Slot == "body" && (region.Width == 0 || region.Height == 0) {
		return delta, errors.New("body region requires positive width and height")
	}
	if exists {
		unchanged := region.Slot == current.Slot && region.Width == current.Width && region.Height == current.Height && region.Row == current.Row && region.Column == current.Column && slices.Equal(region.Lines, current.Lines) && slices.Equal(region.Spans, current.Spans)
		if !unchanged {
			region.Revision++
		}
	} else {
		s.serial++
		region.Order = s.serial
	}
	region.Lines, region.Spans = slices.Clone(region.Lines), slices.Clone(region.Spans)
	s.regions[m.ID] = region
	delete(s.reserved, m.ID)
	region.Lines, region.Spans = slices.Clone(region.Lines), slices.Clone(region.Spans)
	delta.Region = &region
	return delta, nil
}
