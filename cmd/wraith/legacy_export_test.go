package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/session"
	"github.com/erdajt/wraith/internal/store"
	"github.com/erdajt/wraith/internal/variables"
)

func TestLegacyExportPreservesPrivacyWithoutAssigningDirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	directory := t.TempDir()
	env := variables.Store{Root: filepath.Join(root, "config", "variables")}
	state, err := env.Load(directory, false)
	if err != nil {
		t.Fatal(err)
	}
	state, err = env.Save(directory, false, state.Revision, "TARGET=retired.example")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = env.Save(directory, false, state.Revision, "TARGET=current.example"); err != nil {
		t.Fatal(err)
	}
	chat := session.New("test")
	chat.Directory = ""
	chat.Messages = []session.Message{{Role: "user", Content: "retired.example current.example"}}
	sessions := store.Store{Root: root}
	if err := sessions.Save(chat); err != nil {
		t.Fatal(err)
	}
	args := []string{"--config-dir", filepath.Join(root, "config"), "--data-dir", root, "export", chat.ID}
	var out bytes.Buffer
	if err := run(args, &out); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), ".example") || strings.Count(out.String(), "wraith::TARGET") != 2 {
		t.Fatalf("legacy export lost redaction: %s", out.String())
	}
	saved, err := sessions.Load(chat.ID)
	if err != nil || saved.Directory != "" {
		t.Fatalf("export assigned an original directory: %q, %v", saved.Directory, err)
	}
	if _, err := sessions.LoadDirectory(chat.ID, directory); err == nil {
		t.Fatal("export made legacy session resumable")
	}
	if err := os.Remove(filepath.Join(env.Root, "state.json")); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := run(args, &out); err == nil || out.Len() != 0 {
		t.Fatal("legacy export bypassed missing privacy journal")
	}
}
