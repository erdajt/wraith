package distribution

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestBuildExportsReproducibleSelfContainedCandidate(t *testing.T) {
	t.Parallel()
	_, entry, source := packageSeed(t, map[string]string{"LICENSE": "fixture license", "lua/shroud/init.lua": "return {candidate=true}"})
	if _, err := runGit(t.Context(), source, "update-ref", "refs/heads/private-branch", entry.Commit); err != nil {
		t.Fatal(err)
	}
	first, data, err := Build(t.Context(), source, entry)
	if err != nil {
		t.Fatal(err)
	}
	second, repeated, err := Build(t.Context(), source, entry)
	if err != nil || first != second || !bytes.Equal(data, repeated) {
		t.Fatalf("same source/toolchain changed bundle bytes: %v", err)
	}
	if bytes.Contains(data, []byte("private-branch")) {
		t.Fatal("bundle exported unrelated refs")
	}
	seeds := fstest.MapFS{first.Artifact: {Data: data}}
	path, err := Install(t.Context(), seeds, first, t.TempDir(), 2)
	if err != nil {
		t.Fatal(err)
	}
	installed, err := os.ReadFile(filepath.Join(path, "lua/shroud/init.lua"))
	if err != nil || string(installed) != "return {candidate=true}" {
		t.Fatalf("candidate did not survive export/install: %q, %v", installed, err)
	}
	if err := os.WriteFile(filepath.Join(source, "lua/shroud/init.lua"), []byte("uncommitted"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Build(t.Context(), source, entry); err == nil {
		t.Fatal("exported uncommitted source")
	}
}

func TestBuildRejectsShallowHistory(t *testing.T) {
	t.Parallel()
	_, entry, source := packageSeed(t, map[string]string{"LICENSE": "fixture license", "lua/shroud/init.lua": "return {}"})
	if err := os.WriteFile(filepath.Join(source, ".git", "shallow"), []byte(entry.Commit+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Build(t.Context(), source, entry); err == nil {
		t.Fatal("exported shallow history")
	}
}
