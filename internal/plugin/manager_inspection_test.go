package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestManagerInspectionClassifiesFilesystemPaths distinguishes absence from blocked traversal without fetching.
func TestManagerInspectionClassifiesFilesystemPaths(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "directory", "child"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "file"), []byte("ordinary file"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, target := range map[string]string{
		"directory-link": "directory", "dangling": "missing", "loop": "loop",
	} {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	locked := filepath.Join(root, "locked")
	if err := os.Mkdir(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(locked, 0o700); err != nil {
			t.Error(err)
		}
	})

	for _, tc := range []struct {
		name, path, managed, local string
	}{
		{"directory", "directory", "absent", "available"},
		{"missing parents", "missing/parent/child", "absent", "absent"},
		{"file", "file", "blocked", "blocked"},
		{"file ancestor", "file/child", "blocked", "blocked"},
		{"final directory symlink", "directory-link", "blocked", "available"},
		{"ancestor directory symlink", "directory-link/child", "absent", "available"},
		{"absent under directory symlink", "directory-link/missing/child", "absent", "absent"},
		{"dangling symlink", "dangling", "blocked", "blocked"},
		{"dangling ancestor", "dangling/child", "blocked", "blocked"},
		{"symlink loop", "loop", "blocked", "blocked"},
		{"loop ancestor", "loop/child", "blocked", "blocked"},
		{"permission", "locked/child", "blocked", "blocked"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "permission" && os.Geteuid() == 0 {
				t.Skip("root bypasses directory permissions")
			}
			path := filepath.Join(root, tc.path)
			lockfile := filepath.Join(t.TempDir(), "lock.json")
			openManagerRuntime(t, fmt.Sprintf(`local store = require("wraith.manager.store")
local path, lockfile = %q, %q
local git =
	{ name = "tool", source = { kind = "git", url = "/missing/inspection-origin" }, selector = { kind = "head" }, pin = false }
local local_spec =
	{ name = "localtool", source = { kind = "local", dir = path }, selector = { kind = "head" }, pin = false }

for _, specs in ipairs({ { git, local_spec }, { local_spec, git } }) do
	local rows = store.new(specs, { root = path, lockfile = lockfile }).inspect()
	for _, row in ipairs(rows) do
		if row.name == "tool" then
			assert(row.availability == %q, wraith.json.encode(row))
			assert(row.path == nil and row.state == "missing")
			if row.availability == "blocked" then
				assert(type(row.error) == "string" and #row.error > 0)
			end
		else
			assert(row.availability == %q, wraith.json.encode(row))
			assert(row.state == "local")
			if row.availability == "available" then
				assert(row.path == path)
			else
				assert(row.path == nil and type(row.error) == "string")
			end
		end
	end
end`, path, lockfile, tc.managed, tc.local))
			if _, err := os.Stat(lockfile); !os.IsNotExist(err) {
				t.Fatalf("inspection wrote a lock: %v", err)
			}
		})
	}
}

// TestManagerInspectionPreservesAvailableLockSelection rejects managed symlinks without changing revision policy.
func TestManagerInspectionPreservesAvailableLockSelection(t *testing.T) {
	const commit = "0123456789012345678901234567890123456789"
	for _, tc := range []struct {
		name, component, availability string
		conflict, missing             bool
	}{
		{name: "available changed selector and pin", availability: "available"},
		{name: "absent changed selector and pin", availability: "absent", missing: true},
		{name: "source conflict", availability: "blocked", conflict: true},
		{name: "objects symlink", component: "objects", availability: "blocked"},
		{name: "package symlink", component: "objects/tool", availability: "blocked"},
		{name: "checkout symlink", component: "objects/tool/" + commit, availability: "blocked"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			storage, lockfile := filepath.Join(root, "packages"), filepath.Join(root, "lock.json")
			checkout := filepath.Join(storage, "objects", "tool", commit)
			if err := os.MkdirAll(checkout, 0o700); err != nil {
				t.Fatal(err)
			}
			if tc.missing {
				if err := os.Remove(checkout); err != nil {
					t.Fatal(err)
				}
			}
			if tc.component != "" {
				component, target := filepath.Join(storage, tc.component), filepath.Join(root, "moved")
				if err := os.Rename(component, target); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, component); err != nil {
					t.Fatal(err)
				}
			}
			lock := fmt.Sprintf(`{"version":1,"generation":"inspection","packages":{"tool":{"source":{"kind":"git","url":"/missing/inspection-origin"},"selector":{"kind":"branch","value":"old"},"pin":false,"commit":%q}}}`, commit)
			if err := os.WriteFile(lockfile, []byte(lock), 0o600); err != nil {
				t.Fatal(err)
			}

			openManagerRuntime(t, fmt.Sprintf(`local source = { kind = "git", url = "/missing/inspection-origin" }
if %t then
	source.url = "/missing/different-origin"
end

local store = require("wraith.manager.store").new(
	{ {
		name = "tool",
		source = source,
		selector = { kind = "branch", value = "new" },
		pin = true,
	} },
	{ root = %q, lockfile = %q }
)
local rows, lock = store.inspect()
local row = rows[1]
assert(row.availability == %q, wraith.json.encode(row))
assert(row.selector.value == "new" and row.pin and lock.packages.tool.selector.value == "old")
assert(lock.packages.tool.pin == false)

if row.availability == "available" then
	assert(row.path == %q and row.state == "installed" and row.commit == %q)
else
	assert(row.path == nil and type(row.error) == "string" and #row.error > 0)
	if %t then
		assert(row.state == "source-conflict")
	end
end`, tc.conflict, storage, lockfile, tc.availability, checkout, commit, tc.conflict))
			after, err := os.ReadFile(lockfile)
			if err != nil || string(after) != lock {
				t.Fatalf("inspection changed the lock: %v", err)
			}
		})
	}
}
