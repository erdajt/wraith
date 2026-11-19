package distribution

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"testing/fstest"
)

func TestSeedRejectsTamperingAndIncompleteBundles(t *testing.T) {
	t.Parallel()
	entry := fixtureEntry()
	header := "# v2 git bundle\n" + entry.Commit + " refs/wraith/seed\n\nPACKfixture"
	for _, tc := range []struct {
		name string
		data string
	}{
		{"prerequisites", strings.Replace(header, "\n", "\n-"+entry.Commit+" prior\n", 1)},
		{"another commit", strings.Replace(header, entry.Commit, strings.Repeat("c", 40), 1)},
		{"another ref", strings.Replace(header, "refs/wraith/seed", "refs/heads/main", 1)},
		{"extra ref", strings.Replace(header, "\n\n", "\n"+entry.Commit+" refs/heads/other\n\n", 1)},
		{"format", strings.Replace(header, "# v2 git bundle", "# v3 git bundle\n@object-format=sha256", 1)},
		{"missing pack", strings.Replace(header, "PACKfixture", "", 1)},
		{"unknown capability", strings.Replace(header, "# v2 git bundle", "# v3 git bundle\n@filter=blob:none", 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := entry
			candidate.Size, candidate.SHA256 = int64(len(tc.data)), fmt.Sprintf("%x", sha256.Sum256([]byte(tc.data)))
			root := fstest.MapFS{candidate.Artifact: {Data: []byte(tc.data)}}
			if _, err := ReadSeed(root, candidate); err == nil {
				t.Fatal("accepted incomplete or mismatched bundle")
			}
		})
	}
	entry.Size, entry.SHA256 = int64(len(header)), fmt.Sprintf("%x", sha256.Sum256([]byte(header)))
	root := fstest.MapFS{entry.Artifact: {Data: []byte(header)}}
	data, err := ReadSeed(root, entry)
	if err != nil || string(data) != header {
		t.Fatalf("valid seed: %q, %v", data, err)
	}
	root[entry.Artifact].Data[0] = '!'
	if _, err := ReadSeed(root, entry); err == nil {
		t.Fatal("accepted modified bytes")
	}
	if string(data) != header {
		t.Fatal("returned seed aliases mutable storage")
	}
}

func TestSeedRejectsPipeBeforeOpeningIt(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	entry := fixtureEntry()
	entry.Artifact = "seed.bundle"
	if err := syscall.Mkfifo(filepath.Join(directory, entry.Artifact), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if _, err := ReadSeed(root.FS(), entry); err == nil {
		t.Fatal("accepted a pipe as a seed artifact")
	}
}

func TestSeedRejectsSymlinksAndChangedSizes(t *testing.T) {
	t.Parallel()
	entry := fixtureEntry()
	for _, root := range []fstest.MapFS{
		{entry.Artifact: {Data: make([]byte, 100), Mode: fs.ModeSymlink}},
		{"seeds": {Mode: fs.ModeSymlink}, entry.Artifact: {Data: make([]byte, 100)}},
		{entry.Artifact: {Data: make([]byte, 101)}},
		{entry.Artifact: {Mode: fs.ModeDir}},
		{},
	} {
		if _, err := ReadSeed(root, entry); err == nil {
			t.Fatal("accepted unsafe seed path or size")
		}
	}
}
