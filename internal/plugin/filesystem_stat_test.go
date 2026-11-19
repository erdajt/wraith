package plugin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestLuaFilesystemStatDistinguishesMissingPaths(t *testing.T) {
	L := filesystemFixture(t)
	root := L.GetGlobal("root").String()
	if err := os.WriteFile(filepath.Join(root, "file"), []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	for name, target := range map[string]string{"link": "file", "dangling": "missing", "loop": "loop"} {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := L.DoString(`local fs = wraith.fs
local function missing(path, follow)
 local result = fs.stat(path, {missing_ok=true, follow=follow})
 assert(result.path == path and result.exists == false)
 assert(result.type == nil and result.size == nil and result.mode == nil and result.modified_ms == nil)
 assert(not pcall(fs.stat, path, {follow=follow}))
 assert(not pcall(fs.stat, path, {missing_ok=false, follow=follow}))
end
missing(root .. "/missing", false)
missing(root .. "/file/child", false)
missing(root .. "/dangling", true)
for _, name in ipairs({"link", "dangling", "loop"}) do
 local result = fs.stat(root .. "/" .. name, {missing_ok=true})
 assert(result.exists and result.type == "symlink")
end
local result = fs.stat(root .. "/link", {missing_ok=true, follow=true})
assert(result.exists and result.type == "file" and result.size == 4 and result.mode == 384)
assert(type(result.modified_ms) == "number")
assert(fs.stat(root).exists and fs.stat(root).type == "directory")
assert(fs.stat(root .. "/file", {missing_ok=true}).type == "file")
assert(not pcall(fs.stat, root .. "/loop", {missing_ok=true, follow=true}))
assert(not pcall(fs.stat, root, {missing_ok="yes"}))
assert(not pcall(fs.stat_async, root, {missing_ok="yes"}, function() end))
assert(not pcall(fs.read, root, {missing_ok=true}))`); err != nil {
		t.Fatal(err)
	}
	_, err := (fileOperation{kind: "stat", path: filepath.Join(root, "loop"), follow: true, missingOK: true}).run(t.Context())
	if !errors.Is(err, unix.ELOOP) {
		t.Fatalf("symlink loop was classified as missing: %v", err)
	}
}

func TestFileStatPreservesPermissionErrors(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	root := t.TempDir()
	if err := os.Chmod(root, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(root, 0700); err != nil {
			t.Error(err)
		}
	})
	_, err := (fileOperation{kind: "stat", path: filepath.Join(root, "child"), missingOK: true}).run(t.Context())
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("permission failure was classified as missing: %v", err)
	}
}

type statCancellationContext struct {
	context.Context
	cancel context.CancelFunc
	checks int
}

// err cancels at the post-syscall boundary without relying on filesystem timing.
func (c *statCancellationContext) Err() error {
	c.checks++
	if c.checks == 2 {
		c.cancel()
	}
	return c.Context.Err()
}

func TestFileStatCancellationPrecedesResultClassification(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{root, filepath.Join(root, "missing")} {
		for _, after := range []bool{false, true} {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			checked := &statCancellationContext{Context: ctx, cancel: cancel}
			if !after {
				cancel()
			}
			result, err := (fileOperation{kind: "stat", path: path, missingOK: true}).run(checked)
			if !errors.Is(err, context.Canceled) || result != nil {
				t.Fatalf("stat %q after=%t: result=%v error=%v", path, after, result, err)
			}
		}
	}
}

func TestAsyncFileStatReportsMissingAndPresentResults(t *testing.T) {
	root := t.TempDir()
	for _, missing := range []bool{false, true} {
		path := root
		if missing {
			path = filepath.Join(root, "missing")
		}
		r := openTestRuntime(t, fmt.Sprintf(`wraith.fs.stat_async(%q, {missing_ok=true}, function(ctx)
 assert(ctx.data.state == "completed" and ctx.data.error == nil)
 local result = ctx.data.result
 assert(result.path == %q and result.exists == %t)
 if result.exists then
  assert(result.type == "directory" and type(result.size) == "number")
 else
  assert(result.type == nil and result.size == nil and result.mode == nil and result.modified_ms == nil)
 end
 wraith.notify("stat complete")
end)`, path, path, !missing))
		m := r.Manifest()
		if err := r.Activate(m.Resources); err != nil {
			t.Fatal(err)
		}
		pending, err := r.SubmitResource(receiveResource(t, r), nil, m.Config)
		if err != nil {
			t.Fatal(err)
		}
		got := receiveResult(t, pending)
		if got.Err != nil || len(got.Effects) != 1 || got.Effects[0].Argument != "stat complete" {
			t.Fatalf("stat callback: %+v", got)
		}
	}
}
