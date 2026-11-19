package distribution

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckoutVerificationIgnoresIndexFlagsAndReplacementObjects(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"assume-unchanged", "skip-worktree", "replace"} {
		t.Run(kind, func(t *testing.T) {
			_, entry, directory := packageSeed(t, map[string]string{"LICENSE": "fixture license", "lua/shroud/init.lua": "return {value='original'}"})
			if err := verifyCheckout(t.Context(), directory, entry); err != nil {
				t.Fatal(err)
			}
			if kind != "replace" {
				if _, err := runGit(t.Context(), directory, "update-index", "--"+kind, "lua/shroud/init.lua"); err != nil {
					t.Fatal(err)
				}
			}
			file := filepath.Join(directory, "lua/shroud/init.lua")
			if err := os.WriteFile(file, []byte("return {value='modified'}"), 0o600); err != nil {
				t.Fatal(err)
			}
			if kind == "replace" {
				for _, args := range [][]string{
					{"add", "."},
					{"-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "--quiet", "-m", "modified"},
					{"replace", entry.Commit, "HEAD"},
					{"update-ref", "HEAD", entry.Commit},
				} {
					if _, err := runGit(t.Context(), directory, args...); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := verifyCheckout(t.Context(), directory, entry); err == nil {
				t.Fatal("modified source passed exact revision validation")
			}
			data, _ := os.ReadFile(file)
			if string(data) != "return {value='modified'}" {
				t.Fatal("verification changed local source")
			}
		})
	}
}

func TestCheckoutVerificationNeverRunsCleanFilters(t *testing.T) {
	t.Parallel()
	_, entry, directory := packageSeed(t, map[string]string{
		"LICENSE": "fixture license", "lua/shroud/init.lua": "return {value='original'}",
		".gitattributes": "*.lua filter=probe\n",
	})
	if _, err := runGit(t.Context(), directory, "config", "filter.probe.clean", "touch marker; cat"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "lua/shroud/init.lua"), []byte("return {value='modified'}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyCheckout(t.Context(), directory, entry); err == nil {
		t.Fatal("modified filtered source was accepted")
	}
	if _, err := os.Stat(filepath.Join(directory, "marker")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("verification executed a repository filter: %v", err)
	}
}
