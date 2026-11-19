package distribution

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

type swapStoreFS struct {
	fstest.MapFS
	root, outside string
	swapped       bool
}

// ReadDir delegates immutable seed directory reads.
func (s *swapStoreFS) ReadDir(name string) ([]fs.DirEntry, error) { return fs.ReadDir(s.MapFS, name) }

// Open replaces the store parent at a deterministic point after validation.
func (s *swapStoreFS) Open(name string) (fs.File, error) {
	if !s.swapped {
		s.swapped = true
		if err := os.Rename(filepath.Join(s.root, "objects"), filepath.Join(s.root, "old-objects")); err != nil {
			return nil, err
		}
		if err := os.Symlink(s.outside, filepath.Join(s.root, "objects")); err != nil {
			return nil, err
		}
	}
	return s.MapFS.Open(name)
}
func TestInstallRejectsConcurrentManagedParentReplacement(t *testing.T) {
	seeds, entry, _ := packageSeed(t, map[string]string{"LICENSE": "license", "lua/shroud/init.lua": "return {}"})
	root, outside := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "objects"), 0700); err != nil {
		t.Fatal(err)
	}
	malicious := &swapStoreFS{MapFS: seeds, root: root, outside: outside}
	if _, err := Install(t.Context(), malicious, entry, root, 2); err == nil {
		t.Fatal("published outside root after checked managed parent replaced")
	}
	if _, err := os.Stat(filepath.Join(outside, entry.Name, entry.Commit)); err == nil {
		t.Fatal("publication escaped to replacement managed parent")
	}
}
