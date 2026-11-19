package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestManagerVerificationPreservesExistingObjects checks integrity without resetting local changes.
func TestManagerVerificationPreservesExistingObjects(t *testing.T) {
	source, commit := managerGitFixture(t)
	for _, test := range []struct {
		name, expected string
		change         func(*testing.T, string)
	}{
		{"clean", "completed", nil},
		{"tracked", "failed", func(t *testing.T, root string) {
			if err := os.WriteFile(filepath.Join(root, "plugin.lua"), []byte("local changes\n"), 0600); err != nil {
				t.Fatal(err)
			}
		}},
		{"untracked", "failed", func(t *testing.T, root string) {
			if err := os.WriteFile(filepath.Join(root, "notes"), []byte("keep this\n"), 0600); err != nil {
				t.Fatal(err)
			}
		}},
		{"ignored", "failed", func(t *testing.T, root string) {
			if err := os.WriteFile(filepath.Join(root, ".git/info/exclude"), []byte("private\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "private"), []byte("keep this too\n"), 0600); err != nil {
				t.Fatal(err)
			}
		}},
		{"source", "failed", func(t *testing.T, root string) {
			managerGit(t, root, "remote", "set-url", "origin", "/another/source")
		}},
		{"git symlink", "failed", func(t *testing.T, root string) {
			metadata := filepath.Join(t.TempDir(), "metadata")
			if err := os.Rename(filepath.Join(root, ".git"), metadata); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(metadata, filepath.Join(root, ".git")); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "object")
			managerGit(t, "", "clone", "--quiet", "--", source, root)
			if test.change != nil {
				test.change(t, root)
			}
			before := managerGit(t, root, "status", "--porcelain=v1", "--untracked-files=all", "--ignored=matching")
			r := openManagerRuntime(t, fmt.Sprintf(`require("wraith.manager.store_verify").check(
 {source={url=%q},commit=%q},%q,{},function(result)
 assert(result.state==%q,result.error)
 wraith.notify("complete")
end)`, source, commit, root, test.expected))
			runManagerResources(t, r)
			if after := managerGit(t, root, "status", "--porcelain=v1", "--untracked-files=all", "--ignored=matching"); after != before {
				t.Fatalf("verification changed checkout: before %q, after %q", before, after)
			}
		})
	}
}

// TestManagerVerificationRejectsWrongRevisionAndRootSymlink never follows a managed root alias.
func TestManagerVerificationRejectsWrongRevisionAndRootSymlink(t *testing.T) {
	source, commit := managerGitFixture(t)
	root := filepath.Join(t.TempDir(), "object")
	managerGit(t, "", "clone", "--quiet", "--", source, root)
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ root, commit, message string }{
		{root, strings.Repeat("1", 40), "revision differs"},
		{alias, commit, "not a symlink"},
	} {
		r := openManagerRuntime(t, fmt.Sprintf(`require("wraith.manager.store_verify").check(
 {source={url=%q},commit=%q},%q,{},function(result)
 assert(result.state=="failed" and result.error:find(%q,1,true),result.error)
 wraith.notify("complete")
end)`, source, test.commit, test.root, test.message))
		runManagerResources(t, r)
	}
}
