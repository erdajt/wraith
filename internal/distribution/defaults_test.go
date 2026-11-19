package distribution

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultManagerUsesItsDeclaredRepositoryRevision(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	selected, err := Prepare(t.Context(), "shroud", directory, 2)
	if err != nil {
		t.Fatal(err)
	}
	if selected.Entry.Source != "https://github.com/erdajt/shroud.wraith.git" {
		t.Fatalf("unexpected source: %+v", selected.Entry)
	}
	for _, path := range []string{"LICENSE", "lua/shroud/init.lua", "lua/wraith/manager/init.lua", "lua/wraith/manager/store.lua"} {
		if _, err := os.Stat(filepath.Join(selected.Path, path)); err != nil {
			t.Fatalf("missing package source %s: %v", path, err)
		}
	}
	if _, err := Prepare(t.Context(), "shroud", directory, 1); err == nil {
		t.Fatal("loaded incompatible host api")
	}
	if _, err := Prepare(t.Context(), "missing", directory, 2); err == nil {
		t.Fatal("selected an undeclared package")
	}
	reused, err := Prepare(t.Context(), "shroud", directory, 2)
	if err != nil || reused != selected {
		t.Fatalf("changed installed default: %+v, %v", reused, err)
	}
}
