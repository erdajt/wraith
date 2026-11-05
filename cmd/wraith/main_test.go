package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/erdajt/wraith/internal/session"
	"github.com/erdajt/wraith/internal/store"
)

func TestSnapshotIsReadOnlyAndFitsTerminal(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	err := run([]string{"--config-dir", root + "/config", "--data-dir", root + "/data", "--cache-dir", root + "/cache", "--snapshot", "--width", "80", "--height", "24"}, &out)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	if len(lines) != 24 {
		t.Fatalf("got %d rows", len(lines))
	}
	for i, line := range lines {
		if width := ansi.StringWidth(line); width != 80 {
			t.Fatalf("row %d has width %d", i, width)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("snapshot wrote state: %v, %v", entries, err)
	}
}

func TestExportIncludesRecoveredFacts(t *testing.T) {
	root := t.TempDir()
	s := session.New("test-model")
	s.Pins = []string{"target: lab.internal"}
	s.Facts = []string{"tls 1.3 observed"}
	s.Messages = []session.Message{{Role: "assistant", Content: "verified", Edited: true}}
	if err := (store.Store{Root: root}).Save(s); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err := run([]string{"--config-dir", filepath.Join(root, "config"), "--data-dir", root, "export", s.ID}, &out)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{s.ID, "target: lab.internal", "tls 1.3 observed", "edited locally"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("export omitted %q", want)
		}
	}
}

func TestPositionalWordsPopulateDraft(t *testing.T) {
	var out bytes.Buffer
	err := run([]string{"--snapshot", "i", "need", "to", "scan", "192.1.1", "for", "open", "ports"}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ansi.Strip(out.String()), "i need to scan 192.1.1 for open ports") {
		t.Fatal("positional prompt missing")
	}
}

func TestResumeRejectsConflictingSessionFlags(t *testing.T) {
	for _, args := range [][]string{{"--resume", "--new"}, {"--resume", "--session", "123"}} {
		if err := run(args, &bytes.Buffer{}); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
