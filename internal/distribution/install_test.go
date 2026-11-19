package distribution

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestInstallPublishesExactCheckoutAndReusesItOffline(t *testing.T) {
	t.Parallel()
	seeds, entry, _ := packageSeed(t, map[string]string{"LICENSE": "fixture license", "lua/shroud/init.lua": "return {identity='candidate'}"})
	directory := t.TempDir()
	var workers sync.WaitGroup
	for range 4 {
		workers.Go(func() {
			path, err := Install(t.Context(), seeds, entry, directory, 2)
			if err != nil {
				t.Errorf("concurrent install: %v", err)
				return
			}
			data, err := os.ReadFile(filepath.Join(path, "lua/shroud/init.lua"))
			if err != nil || string(data) != "return {identity='candidate'}" {
				t.Errorf("wrong candidate source: %q, %v", data, err)
			}
		})
	}
	workers.Wait()
	path, err := Install(t.Context(), nil, entry, directory, 2)
	if err != nil {
		t.Fatal(err)
	}
	origin, err := runGit(t.Context(), path, "remote", "get-url", "origin")
	if err != nil || origin != entry.Source+"\n" {
		t.Fatalf("origin is not canonical: %q, %v", origin, err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 || entries[0].Name() != entry.Commit {
		t.Fatalf("installation left staging data: %v, %v", entries, err)
	}
}

func TestInstallPreservesModifiedCheckouts(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"tracked", "untracked", "ignored", "origin", "revision", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			seeds, entry, _ := packageSeed(t, map[string]string{"LICENSE": "fixture license", "lua/shroud/init.lua": "return {}", ".gitignore": "ignored\n"})
			directory := t.TempDir()
			path, err := Install(t.Context(), seeds, entry, directory, 2)
			if err != nil {
				t.Fatal(err)
			}
			changed := filepath.Join(path, kind)
			switch kind {
			case "origin":
				_, err = runGit(t.Context(), path, "config", "remote.origin.url", "https://github.com/other/source.git")
			case "revision":
				_, err = runGit(t.Context(), path, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "--allow-empty", "-m", "another revision")
			case "symlink":
				changed = path + "-original"
				if err = os.Rename(path, changed); err == nil {
					err = os.Symlink(changed, path)
				}
			default:
				if kind == "tracked" {
					changed = filepath.Join(path, "lua/shroud/init.lua")
				}
				err = os.WriteFile(changed, []byte("local changes"), 0o600)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Install(t.Context(), seeds, entry, directory, 2); err == nil {
				t.Fatal("reused modified checkout")
			}
			if kind == "tracked" || kind == "untracked" || kind == "ignored" {
				data, _ := os.ReadFile(changed)
				if string(data) != "local changes" {
					t.Fatal("recovery overwrote local changes")
				}
			}
		})
	}
}

func TestInstallRejectsInvalidPackageWithoutPublishing(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"pack", "module", "license", "cancelled"} {
		t.Run(kind, func(t *testing.T) {
			files := map[string]string{"LICENSE": "fixture license", "lua/shroud/init.lua": "return {}"}
			if kind == "module" {
				delete(files, "lua/shroud/init.lua")
			}
			if kind == "license" {
				delete(files, "LICENSE")
			}
			seeds, entry, _ := packageSeed(t, files)
			if kind == "pack" {
				data := seeds[entry.Artifact].Data
				data[len(data)-1] ^= 1
				entry.SHA256 = fmt.Sprintf("%x", sha256.Sum256(data))
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if kind == "cancelled" {
				cancel()
			}
			directory := t.TempDir()
			if _, err := Install(ctx, seeds, entry, directory, 2); err == nil {
				t.Fatal("published invalid package")
			}
			entries, _ := os.ReadDir(filepath.Join(directory, "objects", entry.Name))
			if len(entries) != 0 {
				t.Fatalf("failed install left objects: %v", entries)
			}
		})
	}
}

func TestInstallRejectsRedirectedStoreDirectories(t *testing.T) {
	t.Parallel()
	seeds, entry, _ := packageSeed(t, map[string]string{"LICENSE": "fixture license", "lua/shroud/init.lua": "return {}"})
	for _, component := range []string{".", "objects", "objects/shroud"} {
		t.Run(component, func(t *testing.T) {
			base, outside := t.TempDir(), t.TempDir()
			directory := filepath.Join(base, "store")
			link := filepath.Join(directory, component)
			if err := os.MkdirAll(filepath.Dir(link), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, link); err != nil {
				t.Fatal(err)
			}
			if _, err := Install(t.Context(), seeds, entry, directory, 2); err == nil {
				t.Fatal("installed through a redirected store")
			}
			entries, err := os.ReadDir(outside)
			if err != nil || len(entries) != 0 {
				t.Fatalf("wrote outside the store: %v, %v", entries, err)
			}
		})
	}
}
