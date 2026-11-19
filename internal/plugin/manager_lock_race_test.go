package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestManagerLockReadsConcurrentPublication preserves the published baseline and rejects invalid replacements.
func TestManagerLockReadsConcurrentPublication(t *testing.T) {
	baseline := "  {\"version\":1,\"generation\":\"published.2\",\"packages\":{}}\n"
	for _, test := range []struct {
		name    string
		content string
		error   string
	}{
		{name: "valid", content: baseline},
		{name: "malformed", content: "not json", error: "json"},
		{name: "version", content: `{"version":2,"generation":"new","packages":{}}`, error: "unsupported package lock version"},
		{name: "oversized", content: strings.Repeat(" ", (1<<20)+1), error: "package lock exceeds 1 mib"},
		{name: "directory", error: "package lock must be a regular file"},
		{name: "symlink", error: "package lock must be a regular file"},
	} {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			staged, lock := filepath.Join(directory, "staged"), filepath.Join(directory, "lock.json")
			switch test.name {
			case "directory":
				if err := os.Mkdir(staged, 0700); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				target := filepath.Join(directory, "target")
				if err := os.WriteFile(target, []byte(baseline), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, staged); err != nil {
					t.Fatal(err)
				}
			default:
				if err := os.WriteFile(staged, []byte(test.content), 0600); err != nil {
					t.Fatal(err)
				}
			}

			openTestRuntime(t, fmt.Sprintf(`
local codec = require("wraith.manager.store_lock")
local path, staged, expected = %q, %q, %q
local stat, calls = wraith.fs.stat, 0
wraith.fs.stat = function(name, options)
  if name ~= path then return stat(name, options) end
  calls = calls + 1
  local ok, value = pcall(stat, name, options)
  if calls == 1 then
    assert(not ok, "lock existed before publication")
    assert(os.rename(staged, path))
  end
  if not ok then error(value) end
  return value
end
local ok, value, raw = pcall(codec.read, path)
assert(calls == 2, "publication was not inspected again")
if expected == "" then
  assert(ok, tostring(value))
  assert(value.generation == "published.2" and next(value.packages) == nil)
  assert(raw == %q, "published baseline bytes changed")
else
  assert(not ok and tostring(value):find(expected, 1, true), tostring(value))
  assert(raw == nil, "failed read returned a baseline")
end`, lock, staged, test.error, baseline))
		})
	}
}

// TestManagerLockBoundsConflictingObservations stops retries without inventing an empty baseline.
func TestManagerLockBoundsConflictingObservations(t *testing.T) {
	for _, schedule := range []string{"inspection failure", "repeated removal"} {
		t.Run(schedule, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "lock.json")
			openTestRuntime(t, fmt.Sprintf(`
local codec = require("wraith.manager.store_lock")
local path, schedule = %q, %q
local stat, list, calls, scans = wraith.fs.stat, wraith.fs.list, 0, 0
wraith.fs.stat = function(name, options)
  if name ~= path then return stat(name, options) end
  calls = calls + 1
  assert(calls <= 3, "unbounded inspection retries")
  if schedule == "repeated removal" then
    assert(not pcall(stat, name, options), "lock was not removed")
  end
  if calls == 1 or schedule == "repeated removal" then
    wraith.fs.write(path, wraith.json.encode(codec.empty()))
  end
  error("inspection failure " .. calls)
end
wraith.fs.list = function(name, options)
  scans = scans + 1
  local page = list(name, options)
  if schedule == "repeated removal" then assert(os.remove(path)) end
  return page
end
local ok, err, raw = pcall(codec.read, path)
assert(not ok and tostring(err):find("inspection failure 3", 1, true), tostring(err))
assert(calls == 3 and scans == 3 and raw == nil)
`, path, schedule))
		})
	}
}

// TestManagerLockRejectsFailedParentInspection keeps inaccessible parents distinct from absent locks.
func TestManagerLockRejectsFailedParentInspection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock.json")
	openTestRuntime(t, fmt.Sprintf(`
local codec = require("wraith.manager.store_lock")
wraith.fs.list = function() error("parent access denied") end
local ok, err, raw = pcall(codec.read, %q)
assert(not ok and tostring(err):find("parent access denied", 1, true), tostring(err))
assert(raw == nil)
`, path))
}
