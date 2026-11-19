package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/erdajt/wraith/internal/distribution"
)

// candidateRepository creates an independent package source for importer integration tests.
func candidateRepository(t *testing.T, name, assets string) (string, []string) {
	t.Helper()
	source := t.TempDir()
	for path, text := range map[string]string{"LICENSE": "fixture license", "lua/" + name + "/init.lua": "return {candidate=true}"} {
		path = filepath.Join(source, path)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var commit string
	for _, args := range [][]string{
		{"init", "--quiet", "--template=", source}, {"add", "."},
		{"-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "--quiet", "-m", "candidate"},
		{"remote", "add", "origin", "https://github.com/fixture/" + name + ".git"}, {"rev-parse", "HEAD"},
	} {
		command := exec.CommandContext(t.Context(), "git", args...)
		command.Dir = source
		command.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + source, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null"}
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git: %v: %s", err, output)
		}
		commit = strings.TrimSpace(string(output))
	}
	args := []string{"--source", source, "--output", assets, "--name", name, "--repository", "https://github.com/fixture/" + name + ".git", "--module", name, "--commit", commit}
	return source, args
}

func TestBundleCommandPublishesInstallableCandidate(t *testing.T) {
	t.Parallel()
	assets := t.TempDir()
	source, args := candidateRepository(t, "candidate", assets)
	var output bytes.Buffer
	if err := run(t.Context(), args, &output); err != nil {
		t.Fatal(err)
	}
	manifestBytes, err := os.ReadFile(filepath.Join(assets, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := distribution.Parse(manifestBytes, 2)
	if err != nil || len(manifest.Packages) != 1 {
		t.Fatalf("manifest: %+v, %v", manifest, err)
	}
	root, err := os.OpenRoot(assets)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if _, err := distribution.Install(t.Context(), root.FS(), manifest.Packages[0], t.TempDir(), 2); err != nil {
		t.Fatal(err)
	}
	if err := run(t.Context(), args, &output); err != nil {
		t.Fatal(err)
	}
	repeated, _ := os.ReadFile(filepath.Join(assets, "manifest.json"))
	if !bytes.Equal(manifestBytes, repeated) {
		t.Fatal("reimport changed the selected revision")
	}
	if err := os.WriteFile(filepath.Join(source, "lua/candidate/init.lua"), []byte("uncommitted"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run(t.Context(), args, &output); err == nil {
		t.Fatal("imported a dirty candidate")
	}
	preserved, _ := os.ReadFile(filepath.Join(assets, "manifest.json"))
	if !bytes.Equal(manifestBytes, preserved) {
		t.Fatal("failed import replaced the selected revision")
	}
}

func TestConcurrentImportsPreserveBothPackagePins(t *testing.T) {
	t.Parallel()
	assets := t.TempDir()
	_, first := candidateRepository(t, "first", assets)
	_, second := candidateRepository(t, "second", assets)
	var workers sync.WaitGroup
	for _, args := range [][]string{first, second} {
		workers.Go(func() {
			if err := run(t.Context(), args, io.Discard); err != nil {
				t.Error(err)
			}
		})
	}
	workers.Wait()
	data, err := os.ReadFile(filepath.Join(assets, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := distribution.Parse(data, 2)
	if err != nil || len(manifest.Packages) != 2 {
		t.Fatalf("lost concurrent package: %+v, %v", manifest, err)
	}
	if manifest.Packages[0].Name != "first" || manifest.Packages[1].Name != "second" {
		t.Fatalf("wrong pins: %+v", manifest.Packages)
	}
}
