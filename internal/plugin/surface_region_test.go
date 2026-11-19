package plugin

import "testing"

func TestSurfaceRegionOrderRevisionsAndDisposal(t *testing.T) {
	s := newSurfaceStore("regions")
	lease, _ := s.registerOwner("plugin")
	var ids []string
	for _, owner := range []string{"config", "plugin"} {
		id, _ := s.Reserve(owner, "region")
		frame := SurfaceRegion{Slot: "header", Lines: []string{owner}}
		delta, err := s.Apply(SurfaceMutation{Operation: "region.create", ID: id, Owner: owner, Lease: s.OwnerLease(owner), Region: &frame})
		if err != nil || delta.Region.Revision != 1 {
			t.Fatalf("create: %+v, %v", delta, err)
		}
		ids = append(ids, id)
	}
	index := s.Index()
	if len(index.Regions) != 2 || index.Regions[0].ID != ids[0] || index.Regions[0].Order >= index.Regions[1].Order {
		t.Fatalf("creation order: %+v", index)
	}
	frame, _ := s.Region(ids[1])
	frame.Lines[0] = "updated"
	mutation := SurfaceMutation{Operation: "region.set", ID: ids[1], Owner: "plugin", Lease: lease, Expected: 1, Region: &frame}
	delta, err := s.Apply(mutation)
	if err != nil || delta.Region.Revision != 2 {
		t.Fatalf("update: %+v, %v", delta, err)
	}
	if _, err := s.Apply(mutation); err == nil {
		t.Fatal("stale frame accepted")
	}
	delta.Region.Lines[0] = "detached"
	stored, _ := s.Region(ids[1])
	if stored.Lines[0] != "updated" {
		t.Fatal("delta aliases committed frame")
	}
	s.RevokeOwner("plugin")
	if remaining := s.Index().Regions; len(remaining) != 1 || remaining[0].ID != ids[0] {
		t.Fatalf("previous frame not restored: %+v", remaining)
	}
}

func TestSurfaceRegionRejectsInvalidFramesWithoutConsumingReservation(t *testing.T) {
	s := newSurfaceStore("regions")
	id, _ := s.Reserve("config", "region")
	for _, frame := range []SurfaceRegion{
		{Slot: "unknown"}, {Slot: "header", Lines: []string{"a", "b"}},
		{Slot: "sidebar"}, {Slot: "body", Width: 10},
		{Slot: "footer", Lines: []string{"\x1b[31m"}},
	} {
		if _, err := s.Apply(SurfaceMutation{Operation: "region.create", ID: id, Owner: "config", Lease: s.OwnerLease("config"), Region: &frame}); err == nil {
			t.Fatalf("invalid frame accepted: %+v", frame)
		}
	}
	frame := SurfaceRegion{Slot: "footer", Lines: []string{"ready"}}
	if _, err := s.Apply(SurfaceMutation{Operation: "region.create", ID: id, Owner: "config", Lease: s.OwnerLease("config"), Region: &frame}); err != nil {
		t.Fatal(err)
	}
}
