package plugin

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestAtomicFileWritesPreserveDestinationOnFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings")
	op := fileOperation{kind: "write", path: path, data: []byte("original"), mode: 0600}
	if _, err := op.run(t.Context()); err != nil {
		t.Fatal(err)
	}
	op.data = []byte("replacement")
	if _, err := op.run(t.Context()); !errors.Is(err, os.ErrExist) {
		t.Fatalf("no-overwrite result: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "original" {
		t.Fatalf("failed write changed original: %q, %v", data, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	op.overwrite = true
	if _, err := op.run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled write: %v", err)
	}
	if _, err := op.run(t.Context()); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(path)
	if err != nil || string(data) != "replacement" {
		t.Fatalf("replacement: %q, %v", data, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("write permissions: %v, %v", info, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary files leaked: %v, %v", entries, err)
	}
}

func TestFileReadsPreserveBytesAndBoundWindows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bytes")
	want := []byte{0, 255, '\r', '\n', 0xc3, 0xa9}
	if err := os.WriteFile(path, want, 0600); err != nil {
		t.Fatal(err)
	}
	op := fileOperation{kind: "read", path: path, offset: 1, limit: 3}
	got, err := op.run(t.Context())
	if err != nil || !bytes.Equal(got["data"].([]byte), want[1:4]) || got["next_offset"] != int64(4) || got["truncated"] != true {
		t.Fatalf("read window: %+v, %v", got, err)
	}
	op.offset = 4
	got, err = op.run(t.Context())
	if err != nil || !bytes.Equal(got["data"].([]byte), want[4:]) || got["truncated"] != false {
		t.Fatalf("last window: %+v, %v", got, err)
	}
	fifo := filepath.Join(filepath.Dir(path), "fifo")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	op.path = fifo
	if _, err := op.run(t.Context()); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("special file read accepted: %v", err)
	}
}

func TestDirectoryPagingAndRenameDoNotLoseEntries(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"c", "a", "b"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	op := fileOperation{kind: "list", path: dir, limit: 2}
	seen := map[string]bool{}
	for {
		got, err := op.run(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range got["entries"].([]any) {
			name := entry.(map[string]any)["name"].(string)
			if seen[name] {
				t.Fatalf("directory entry repeated: %s", name)
			}
			seen[name] = true
		}
		if !got["truncated"].(bool) {
			break
		}
		op.offset = got["next_offset"].(int64)
	}
	if len(seen) != 3 {
		t.Fatalf("directory entries lost: %v", seen)
	}
	op = fileOperation{kind: "rename", path: filepath.Join(dir, "a"), destination: filepath.Join(dir, "b")}
	if _, err := op.run(t.Context()); !errors.Is(err, os.ErrExist) {
		t.Fatalf("rename replaced existing file: %v", err)
	}
	op.destination = filepath.Join(dir, "renamed")
	if _, err := op.run(t.Context()); err != nil {
		t.Fatal(err)
	}
}
