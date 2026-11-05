package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/session"
)

func TestResumeIsScopedToOriginalDirectory(t *testing.T) {
	s := Store{Root: t.TempDir()}
	one, two := t.TempDir(), t.TempDir()
	a, b := session.New("local"), session.New("local")
	a.Directory, b.Directory = one, two
	for _, chat := range []session.Session{a, b} {
		if err := s.Save(chat); err != nil {
			t.Fatal(err)
		}
	}
	list, err := s.ListDirectory(one)
	if err != nil || len(list) != 1 || list[0].ID != a.ID {
		t.Fatalf("directory picker: %v %v", list, err)
	}
	if _, err := s.LoadDirectory(b.ID, one); err == nil || !strings.Contains(err.Error(), two) {
		t.Fatalf("wrong directory error: %v", err)
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(one, alias); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LoadDirectory(a.ID, alias); err != nil {
		t.Fatal(err)
	}
}
