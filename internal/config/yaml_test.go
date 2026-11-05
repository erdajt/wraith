package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestYAMLMigrationPreservesLegacyPreferences(t *testing.T) {
	dir := t.TempDir()
	c := Default()
	c.Editor = []string{"vim", "-n"}
	if err := Save(filepath.Join(dir, "config.json"), c); err != nil {
		t.Fatal(err)
	}
	got, err := LoadDirectory(dir)
	if err != nil || got.Editor[0] != "vim" {
		t.Fatalf("migration: %v", err)
	}
	got.Theme = "ice"
	if err := Save(File(dir), got); err != nil {
		t.Fatal(err)
	}
	got, err = LoadDirectory(dir)
	if err != nil || got.Theme != "ice" {
		t.Fatal("yaml did not take precedence")
	}
	if _, err := os.Stat(filepath.Join(dir, "config.json")); err != nil {
		t.Fatal("legacy config removed")
	}
}

func TestYAMLRejectsUnknownAndDuplicateFields(t *testing.T) {
	for _, source := range []string{"unknown: yes\n", "theme: ice\ntheme: ember\n", "theme: ice\n---\ntheme: ember\n"} {
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Fatalf("accepted %q", source)
		}
	}
}

func TestExplicitBindingsReplaceDefaultSequences(t *testing.T) {
	path := File(t.TempDir())
	if err := os.WriteFile(path, []byte("leader: ','\nbindings:\n  x y: settings\n"), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil || len(c.Bindings) != 1 || c.Bindings["x y"] != "settings" {
		t.Fatalf("bindings merged unexpectedly: %v %v", c.Bindings, err)
	}
}
