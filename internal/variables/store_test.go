package variables

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreScopesRotationsAndExternalChanges(t *testing.T) {
	t.Parallel()
	s := Store{Root: t.TempDir()}
	dir := t.TempDir()
	global, err := s.Load(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Save(dir, true, global.Revision, "TARGET=global.test"); err != nil {
		t.Fatal(err)
	}
	local, err := s.Load(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Save(dir, false, local.Revision, "TARGET=local.test"); err != nil {
		t.Fatal(err)
	}
	local, err = s.Load(dir, false)
	if err != nil || local.Snapshot.values["TARGET"] != "local.test" {
		t.Fatalf("override: %v", err)
	}
	if err := os.WriteFile(s.Path(dir, false), []byte("TARGET=rotated.test"), 0600); err != nil {
		t.Fatal(err)
	}
	reloaded, err := s.Load(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.Snapshot.Redact("global.test local.test rotated.test"); got != "wraith::TARGET wraith::TARGET wraith::TARGET" {
		t.Fatal(got)
	}
	if _, err := s.Save(dir, false, local.Revision, "TARGET=stale.test"); err == nil {
		t.Fatal("stale editor overwrote changed file")
	}
	other, err := s.Load(t.TempDir(), false)
	if err != nil || other.Snapshot.values["TARGET"] != "global.test" {
		t.Fatalf("scope leaked: %v", err)
	}
	info, err := os.Stat(filepath.Join(s.Root, "state.json"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("journal permissions: %v", err)
	}
}

func TestStoreRejectsCorruptionAndRetainsOriginalOnInvalidEdit(t *testing.T) {
	t.Parallel()
	s := Store{Root: t.TempDir()}
	dir := t.TempDir()
	state, err := s.Load(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	state, err = s.Save(dir, false, state.Revision, "TARGET=private.test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Save(dir, false, state.Revision, "TARGET=\"private.test"); err == nil || strings.Contains(err.Error(), "private.test") {
		t.Fatalf("invalid edit: %v", err)
	}
	if err := os.Remove(filepath.Join(s.Root, "state.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(dir, false); err == nil {
		t.Fatal("missing accepted snapshot was ignored")
	}
}

func TestConcurrentVariableSavesRejectStaleRevisions(t *testing.T) {
	t.Parallel()
	s := Store{Root: t.TempDir()}
	dir := t.TempDir()
	state, err := s.Load(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, text := range []string{"TARGET=one.test", "TARGET=two.test"} {
		go func() { <-start; _, err := s.Save(dir, false, state.Revision, text); results <- err }()
	}
	close(start)
	first, second := <-results, <-results
	if (first == nil) == (second == nil) {
		t.Fatalf("expected one accepted edit, got %v and %v", first, second)
	}
}
