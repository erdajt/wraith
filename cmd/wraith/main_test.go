package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/session"
	"github.com/erdajt/wraith/internal/store"
	"github.com/erdajt/wraith/internal/variables"
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

func TestSessionsPrintsHealthyChatsBeforeReportingDamagedFiles(t *testing.T) {
	root := t.TempDir()
	s := session.New("test-model")
	s.Title = "healthy chat"
	if err := (store.Store{Root: root}).Save(s); err != nil {
		t.Fatal(err)
	}
	broken := session.ID()
	if err := os.WriteFile(filepath.Join(root, "sessions", broken+".json.gz"), []byte("not a gzip stream"), 0600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err := run([]string{"--config-dir", filepath.Join(root, "config"), "--data-dir", root, "sessions"}, &out)
	if err == nil || !strings.Contains(err.Error(), broken) || !strings.Contains(out.String(), s.ID) || !strings.Contains(out.String(), s.Title) {
		t.Fatalf("partial cli listing lost data or warning: %q, %v", out.String(), err)
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

func TestRemovedNewFlagReportsUnknownOption(t *testing.T) {
	err := run([]string{"--new"}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "flag provided but not defined") {
		t.Fatalf("removed flag accepted: %v", err)
	}
}

func TestExportRedactsOriginalDirectoryAndRetiredValues(t *testing.T) {
	root := t.TempDir()
	s := session.New("test")
	s.Directory = t.TempDir()
	s.Messages = []session.Message{{Role: "user", Content: "retired.example current.example"}}

	if err := (store.Store{Root: root}).Save(s); err != nil {
		t.Fatal(err)
	}

	env := variables.Store{Root: filepath.Join(root, "config", "variables")}
	state, err := env.Load(s.Directory, variables.Directory)
	if err != nil {
		t.Fatal(err)
	}

	state, err = env.Save(s.Directory, variables.Directory, state.Revision, "TARGET=retired.example")
	if err != nil {
		t.Fatal(err)
	}

	if _, err = env.Save(s.Directory, variables.Directory, state.Revision, "TARGET=current.example"); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	args := []string{"--config-dir", filepath.Join(root, "config"), "--data-dir", root, "export", s.ID}
	if err := run(args, &out); err != nil {
		t.Fatal(err)
	}

	if strings.Contains(out.String(), ".example") || !strings.Contains(out.String(), "wraith::TARGET") {
		t.Fatalf("private export: %s", out.String())
	}

	if err := os.Remove(filepath.Join(env.Root, "state.json")); err != nil {
		t.Fatal(err)
	}

	out.Reset()

	if err := run(args, &out); err == nil || out.Len() != 0 {
		t.Fatal("export ignored missing redaction journal")
	}
}

func TestDiscoveryCommandsLoadPrivacyBeforeProviderAccess(t *testing.T) {
	root := t.TempDir()
	directory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	c := config.Default()
	c.Provider = "openai"
	profile := c.Providers[c.Provider]
	profile.APIKey, profile.KeyEnv = "private.example", ""
	c.Providers[c.Provider] = profile

	if err := config.Save(filepath.Join(root, "config.yaml"), c); err != nil {
		t.Fatal(err)
	}

	env := variables.Store{Root: filepath.Join(root, "variables")}
	state, err := env.Load(directory, variables.Directory)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.Save(directory, variables.Directory, state.Revision, "TARGET=private.example"); err != nil {
		t.Fatal(err)
	}

	for _, command := range []string{"models", "doctor"} {
		err := run([]string{"--config-dir", root, command}, &bytes.Buffer{})
		if err == nil || !strings.Contains(err.Error(), "private variable collides") {
			t.Fatalf("%s bypassed privacy: %v", command, err)
		}
	}
}

func TestExportLoadsExternallyEditedConversationBindings(t *testing.T) {
	root := t.TempDir()
	chat := session.New("test")
	chat.Messages = []session.Message{{Role: "user", Content: "new-private.example"}}

	if err := (store.Store{Root: root}).Save(chat); err != nil {
		t.Fatal(err)
	}

	env := variables.Store{Root: filepath.Join(root, "config", "variables"), Conversation: chat.ID}
	path := env.Path(chat.Directory, variables.Conversation)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("TARGET=new-private.example"), 0600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := run([]string{"--config-dir", filepath.Join(root, "config"), "--data-dir", root, "export", chat.ID}, &out); err != nil {
		t.Fatal(err)
	}

	if strings.Contains(out.String(), "new-private.example") || !strings.Contains(out.String(), "wraith::TARGET") {
		t.Fatal("export ignored conversation bindings")
	}
}
