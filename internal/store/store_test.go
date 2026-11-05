package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/session"
)

func TestArchiveSurvivesActiveReplacement(t *testing.T) {
	s := Store{Root: t.TempDir()}
	v := session.New("local")
	v.Messages = []session.Message{{Role: "user", Content: strings.Repeat("exact evidence 10.0.0.7:443\n", 100)}}
	if err := s.Save(v); err != nil {
		t.Fatal(err)
	}
	name, err := s.Archive(v)
	if err != nil {
		t.Fatal(err)
	}
	v.Messages = nil
	v.Summary = "shorter"
	if err = s.Save(v); err != nil {
		t.Fatal(err)
	}
	old, err := s.ReadArchive(name)
	if err != nil {
		t.Fatal(err)
	}
	if len(old.Messages) != 1 || !strings.Contains(old.Messages[0].Content, "10.0.0.7:443") {
		t.Fatal("archive lost evidence")
	}
	active, err := s.Load(v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if active.Summary != "shorter" || len(active.Messages) != 0 {
		t.Fatal("active session not replaced")
	}
	b, err := os.ReadFile(filepath.Join(s.Root, "archives", name))
	if err != nil {
		t.Fatal(err)
	}
	if len(b) < 2 || b[0] != 0x1f || b[1] != 0x8b {
		t.Fatal("archive is not gzip")
	}
}

func TestLockAndTraversal(t *testing.T) {
	s := Store{Root: t.TempDir()}
	unlock, err := s.Lock()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Lock(); err == nil {
		t.Fatal("concurrent writer accepted")
	}
	unlock()
	unlock, err = s.Lock()
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if _, err = s.Load("../../secret"); err == nil {
		t.Fatal("path traversal accepted")
	}
	if _, err = s.ReadArchive("../secret"); err == nil {
		t.Fatal("archive traversal accepted")
	}
}

func TestCorruptGzipFails(t *testing.T) {
	s := Store{Root: t.TempDir()}
	v := session.New("local")
	if err := s.Save(v); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(s.Root, "sessions", v.ID+".json.gz")
	b, _ := os.ReadFile(p)
	b[len(b)-1] ^= 0xff
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(v.ID); err == nil {
		t.Fatal("corrupt archive accepted")
	}
}
