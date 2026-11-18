package store

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestInputHistoryIsCompressedAndDirectoryScoped(t *testing.T) {
	s := Store{Root: t.TempDir()}
	directory := t.TempDir()
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(directory, alias); err != nil {
		t.Fatal(err)
	}
	entries := []string{"scan configured target", "/models", "界\nsecond line"}
	if err := s.SaveInputs(directory, entries); err != nil {
		t.Fatal(err)
	}
	got, err := s.Inputs(alias)
	if err != nil || !slices.Equal(got, entries) {
		t.Fatalf("canonical directory history lost: %q, %v", got, err)
	}
	if other, err := s.Inputs(t.TempDir()); err != nil || len(other) != 0 {
		t.Fatal("history crossed directories")
	}
	data, err := os.ReadFile(s.inputPath(directory))
	if err != nil || !bytes.HasPrefix(data, []byte{0x1f, 0x8b}) {
		t.Fatal("input history is not compressed")
	}
	info, err := os.Stat(s.inputPath(directory))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("history permissions expose input")
	}
}

func TestInputHistoryRejectsCorruptionAndOversizedContent(t *testing.T) {
	for _, kind := range []string{"gzip", "checksum", "truncated", "json", "oversized"} {
		t.Run(kind, func(t *testing.T) {
			s := Store{Root: t.TempDir()}
			directory := t.TempDir()
			if err := s.SaveInputs(directory, []string{"retained"}); err != nil {
				t.Fatal(err)
			}
			var encoded bytes.Buffer
			z := gzip.NewWriter(&encoded)
			payload := "invalid json"
			if kind == "oversized" {
				payload = `["` + strings.Repeat("x", 4<<20) + `"]`
			}
			if _, err := z.Write([]byte(payload)); err != nil {
				t.Fatal(err)
			}
			if err := z.Close(); err != nil {
				t.Fatal(err)
			}
			data := encoded.Bytes()
			if kind == "gzip" {
				data = []byte("not gzip")
			} else if kind == "checksum" {
				data[len(data)-1] ^= 0xff
			} else if kind == "truncated" {
				data = data[:len(data)-4]
			}
			if err := os.WriteFile(s.inputPath(directory), data, 0600); err != nil {
				t.Fatal(err)
			}
			_, err := s.Inputs(directory)
			if err == nil {
				t.Fatal("unreadable history accepted")
			}
			if kind == "checksum" && !errors.Is(err, gzip.ErrChecksum) || kind == "truncated" && !errors.Is(err, io.ErrUnexpectedEOF) || kind == "oversized" && !strings.Contains(err.Error(), "decompression limit") {
				t.Fatalf("wrong failure for %s: %v", kind, err)
			}
		})
	}
}

func TestInputHistoryReportsStorageFailures(t *testing.T) {
	root := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(root, nil, 0600); err != nil {
		t.Fatal(err)
	}
	s := Store{Root: root}
	if err := s.SaveInputs(t.TempDir(), []string{"keep"}); err == nil {
		t.Fatal("failed history save reported success")
	}
	if _, err := s.Inputs(t.TempDir()); err == nil {
		t.Fatal("failed history read reported an empty history")
	}
}
