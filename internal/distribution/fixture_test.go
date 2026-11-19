package distribution

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// packageSeed commits a real package and bundles only its named distribution ref.
func packageSeed(t *testing.T, files map[string]string) (fstest.MapFS, Entry, string) {
	t.Helper()
	directory := t.TempDir()
	for path, content := range files {
		path = filepath.Join(directory, path)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{
		{"init", "--quiet", "--template=", "."}, {"add", "."},
		{"-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "--quiet", "-m", "package fixture"},
		{"update-ref", "refs/wraith/seed", "HEAD"},
		{"config", "remote.origin.url", fixtureEntry().Source},
	} {
		if _, err := runGit(t.Context(), directory, arguments...); err != nil {
			t.Fatal(err)
		}
	}
	commit, err := runGit(t.Context(), directory, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(t.TempDir(), "seed.bundle")
	if _, err := runGit(t.Context(), directory, "bundle", "create", "--version=2", artifact, "refs/wraith/seed"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatal(err)
	}
	entry := fixtureEntry()
	entry.Commit, entry.Size, entry.SHA256 = strings.TrimSpace(commit), int64(len(data)), fmt.Sprintf("%x", sha256.Sum256(data))
	return fstest.MapFS{entry.Artifact: {Data: data}}, entry, directory
}
