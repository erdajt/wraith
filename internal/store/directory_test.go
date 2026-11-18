package store

import (
	"compress/gzip"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/session"
)

func TestListingRetainsHealthySessionsAndReportsDamagedFiles(t *testing.T) {
	s := Store{Root: t.TempDir()}
	current, other := t.TempDir(), t.TempDir()
	healthy, foreign := session.New("local"), session.New("local")
	healthy.Directory, foreign.Directory = current, other
	for _, chat := range []session.Session{healthy, foreign} {
		if err := s.Save(chat); err != nil {
			t.Fatal(err)
		}
	}
	damaged := filepath.Join(s.Root, "sessions", foreign.ID+".json.gz")
	if err := os.WriteFile(damaged, []byte("not a gzip stream"), 0600); err != nil {
		t.Fatal(err)
	}
	mismatch := session.ID()
	if err := compressed(filepath.Join(s.Root, "sessions", mismatch+".json.gz"), healthy); err != nil {
		t.Fatal(err)
	}

	list, err := s.ListDirectory(current)
	if len(list) != 1 || list[0].ID != healthy.ID || !errors.Is(err, gzip.ErrHeader) {
		t.Fatalf("partial listing lost valid session or error: %+v, %v", list, err)
	}
	for _, name := range []string{foreign.ID, mismatch} {
		if !strings.Contains(err.Error(), name) {
			t.Fatalf("damaged filename omitted: %s", name)
		}
	}
	if _, err := s.LoadDirectory(healthy.ID, current); err != nil {
		t.Fatal(err)
	}
	if contents, err := os.ReadFile(damaged); err != nil || string(contents) != "not a gzip stream" {
		t.Fatal("listing modified damaged evidence")
	}
	if list, err := s.ListDirectory(other); len(list) != 0 || err == nil {
		t.Fatal("fully damaged directory hid its error or leaked another chat")
	}
}

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
