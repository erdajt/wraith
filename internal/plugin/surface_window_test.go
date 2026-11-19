package plugin

import "testing"

func TestSurfaceWindowsShareTextButKeepIndependentCursors(t *testing.T) {
	store := newSurfaceStore("fixture")
	buffer, err := store.Reserve("config", "buffer")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.HostApply(SurfaceMutation{Operation: "buffer.create", ID: buffer, Buffer: &SurfaceBuffer{Lines: []string{"界 hello"}}}); err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, column := range []int{0, 2} {
		id, err := store.Reserve("config", "window")
		if err != nil {
			t.Fatal(err)
		}
		delta, err := store.HostApply(SurfaceMutation{Operation: "window.create", ID: id, Window: &SurfaceWindow{BufferID: buffer, Cursor: SurfacePosition{0, column}, Focus: true}})
		if err != nil || delta.Window.Revision != 1 || !delta.Window.Focus {
			t.Fatalf("create: %+v %v", delta, err)
		}
		stored, err := store.Window(id)
		if err != nil || stored.Focus || stored.Cursor.Column != column {
			t.Fatalf("stored view: %+v %v", stored, err)
		}
		ids = append(ids, id)
	}
	first, _ := store.Window(ids[0])
	first.Cursor.Column = 1
	changed, err := store.HostApply(SurfaceMutation{Operation: "window.configure", ID: first.ID, Expected: 1, Window: &first})
	if err != nil || changed.Window.Revision != 2 {
		t.Fatalf("cursor update: %+v %v", changed, err)
	}
	if _, err := store.HostApply(SurfaceMutation{Operation: "window.configure", ID: first.ID, Expected: 1, Window: &first}); err == nil {
		t.Fatal("stale layout update accepted")
	}
	second, _ := store.Window(ids[1])
	if second.Cursor.Column != 2 {
		t.Fatal("view cursors were shared")
	}
	if _, err := store.HostApply(SurfaceMutation{Operation: "window.delete", ID: first.ID, Expected: 2}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Buffer(buffer); err != nil {
		t.Fatal("closing a view deleted shared text")
	}
	second.Kind = "float"
	if _, err := store.HostApply(SurfaceMutation{Operation: "window.configure", ID: second.ID, Expected: 1, Window: &second}); err == nil {
		t.Fatal("zero-size floating window accepted")
	}
	second.Width, second.Height = 40, 12
	if _, err := store.HostApply(SurfaceMutation{Operation: "window.configure", ID: second.ID, Expected: 1, Window: &second}); err != nil {
		t.Fatal(err)
	}
}
