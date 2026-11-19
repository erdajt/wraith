package distribution

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPackageStoreStaysAttachedToItsOpenedDirectory(t *testing.T) {
	t.Parallel()
	directory, outside := t.TempDir(), t.TempDir()
	store, err := openStore(directory, "shroud")
	if err != nil {
		t.Fatal(err)
	}
	defer store.close()
	if err := store.sync(); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(directory, "objects"), filepath.Join(directory, "old-objects")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(directory, "objects")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(descriptorPath(store.objects), "marker"), []byte("owned"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(directory, "old-objects", "shroud", "marker")); err != nil {
		t.Fatal("directory handle followed the replacement")
	}
	if err := store.sync(); err == nil {
		t.Fatal("reported a redirected store as current")
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("wrote through replacement: %v, %v", entries, err)
	}
}
