package fsutil

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestFailedReplacementRetainsOriginalAndRemovesTemporaryFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state")
	if err := Bytes(path, []byte("original")); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("disk failure")
	err := Atomic(path, func(w io.Writer) error { _, _ = w.Write([]byte("incomplete")); return failure })
	if !errors.Is(err, failure) {
		t.Fatal("write failure was swallowed")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "original" {
		t.Fatal("original replaced after failed write")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatal("temporary file leaked")
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("private mode lost")
	}
	if _, err := ReadLimit(path, 3); err == nil {
		t.Fatal("oversized state accepted")
	}
}
