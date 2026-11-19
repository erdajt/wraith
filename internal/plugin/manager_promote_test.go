package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestManagerPromotionNeverReplacesExistingObjects checks new, reusable and conflicting destinations.
func TestManagerPromotionNeverReplacesExistingObjects(t *testing.T) {
	source, commit := managerGitFixture(t)
	for _, state := range []string{"missing", "clean", "dirty", "symlink"} {
		t.Run(state, func(t *testing.T) {
			root, stage := t.TempDir(), filepath.Join(t.TempDir(), "stage")
			managerGit(t, "", "clone", "--quiet", "--", source, stage)
			parent := filepath.Join(root, "objects", "app")
			if err := os.MkdirAll(parent, 0700); err != nil {
				t.Fatal(err)
			}
			destination := filepath.Join(parent, commit)
			if state == "clean" || state == "dirty" {
				managerGit(t, "", "clone", "--quiet", "--", source, destination)
			}
			if state == "dirty" {
				if err := os.WriteFile(filepath.Join(destination, "plugin.lua"), []byte("keep local edit"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if state == "symlink" {
				if err := os.Symlink(stage, destination); err != nil {
					t.Fatal(err)
				}
			}
			expected := "completed"
			if state == "dirty" || state == "symlink" {
				expected = "failed"
			}
			r := openManagerRuntime(t, fmt.Sprintf(`require("wraith.manager.store_promote").install(
 {name="app",source={url=%q},commit=%q},%q,%q,{},function(result)
 assert(result.state==%q,result.error)
 if result.state=="completed" then assert(result.path==%q and result.reused==%t) end
 wraith.notify("complete")
end)`, source, commit, stage, root, expected, destination, state == "clean"))
			runManagerResources(t, r)
			_, err := os.Stat(stage)
			if state == "missing" && !os.IsNotExist(err) || state != "missing" && err != nil {
				t.Fatalf("unexpected stage disposition: %v", err)
			}
			if state == "dirty" {
				data, err := os.ReadFile(filepath.Join(destination, "plugin.lua"))
				if err != nil || string(data) != "keep local edit" {
					t.Fatalf("existing edit changed: %q, %v", data, err)
				}
			}
		})
	}
}

// TestManagerPromotionRejectsParentSymlinks keeps writes inside the selected managed root.
func TestManagerPromotionRejectsParentSymlinks(t *testing.T) {
	root, outside, stage := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "objects")); err != nil {
		t.Fatal(err)
	}
	r := openManagerRuntime(t, fmt.Sprintf(`require("wraith.manager.store_promote").install(
 {name="app",source={url="/unused"},commit=string.rep("1",40)},%q,%q,{},function(result)
 assert(result.state=="failed" and result.error:find("not a symlink",1,true))
 wraith.notify("complete")
end)`, stage, root))
	runManagerResources(t, r)
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("wrote through parent symlink: %v, %v", entries, err)
	}
}
