package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestManagerStorageOptionsValidateWithoutCreatingDirectories keeps configuration passive and detached.
func TestManagerStorageOptionsValidateWithoutCreatingDirectories(t *testing.T) {
	root := filepath.Join(t.TempDir(), "absent")
	openManagerRuntime(t, fmt.Sprintf(`local options=require("wraith.manager.store_options")
 local original={root=%q,git_env={GIT_ASKPASS="/user/askpass"}}
 local result=options.normalize(original)
 original.git_env.GIT_ASKPASS="changed"
 assert(result.git_env.GIT_ASKPASS=="/user/askpass" and result.root==%q)
 assert(result.timeout_ms==300000 and result.lock_timeout_ms==5000)
 assert(result.lockfile==wraith.paths.stdpath("config").."/wraith-lock.json")
 for _,bad in ipairs({{root=false},{lockfile=false},{root="relative"},{root="/tmp/a;module"},{lockfile="/"},{timeout_ms=0},{timeout_ms=1.5},{timeout_ms=86400001},{lock_timeout_ms=300001},{git_env={GIT_CONFIG_GLOBAL="/tmp/config"}},{unknown=true}}) do
  assert(not pcall(options.normalize,bad))
 end`, root, root))
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("configuration created package root: %v", err)
	}
}

// TestManagerInspectionDetachesRowsAndRejectsUnavailableRoots never starts git or fills missing installs.
func TestManagerInspectionDetachesRowsAndRejectsUnavailableRoots(t *testing.T) {
	directory := t.TempDir()
	commit := strings.Repeat("1", 40)
	object := filepath.Join(directory, "objects", "installed", commit)
	if err := os.MkdirAll(object, 0700); err != nil {
		t.Fatal(err)
	}
	lockfile := filepath.Join(directory, "lock.json")
	data := fmt.Sprintf(`{"version":1,"generation":"fixture","packages":{"installed":{"source":{"kind":"git","url":"/repo"},"selector":{"kind":"head"},"pin":false,"commit":%q},"changed":{"source":{"kind":"git","url":"/old"},"selector":{"kind":"head"},"pin":false,"commit":%q}}}`, commit, commit)
	if err := os.WriteFile(lockfile, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	openManagerRuntime(t, fmt.Sprintf(`local specs={
 {name="installed",source={kind="git",url="/repo"},selector={kind="head"},pin=false},
 {name="changed",source={kind="git",url="/new"},selector={kind="head"},pin=false},
 {name="missing",source={kind="git",url="/missing"},selector={kind="head"},pin=false},
 {name="local",source={kind="local",dir=%q},selector={kind="head"},pin=false}}
 local rows,lock,raw=require("wraith.manager.store_inspect").read(specs,{root=%q,lockfile=%q})
 assert(rows[1].state=="installed" and rows[1].path==%q)
 assert(rows[2].state=="source-conflict" and not rows[2].path)
 assert(rows[3].state=="missing" and not rows[3].path)
 assert(rows[4].state=="local" and rows[4].path==%q)
 rows[1].source.url="changed"; assert(specs[1].source.url=="/repo")
 assert(lock.generation=="fixture" and raw:find("fixture",1,true))
 local empty=require("wraith.manager.store_inspect").read({},{root=%q,lockfile=%q})
 assert(wraith.json.encode(empty)=="[]")`, directory, directory, lockfile, object, directory, directory, lockfile))
}

// TestManagerInspectionRefusesManagedParentAliases never selects a checkout through a symlinked objects root.
func TestManagerInspectionRefusesManagedParentAliases(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	commit := strings.Repeat("2", 40)
	if err := os.MkdirAll(filepath.Join(outside, "app", commit), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "objects")); err != nil {
		t.Fatal(err)
	}
	lockfile := filepath.Join(root, "lock.json")
	data := fmt.Sprintf(`{"version":1,"generation":"fixture","packages":{"app":{"source":{"kind":"git","url":"/repo"},"selector":{"kind":"head"},"pin":false,"commit":%q}}}`, commit)
	if err := os.WriteFile(lockfile, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	openManagerRuntime(t, fmt.Sprintf(`local rows=require("wraith.manager.store_inspect").read(
 {{name="app",source={kind="git",url="/repo"},selector={kind="head"},pin=false}},{root=%q,lockfile=%q})
 assert(rows[1].state=="missing" and not rows[1].path and rows[1].error)`, root, lockfile))
}
