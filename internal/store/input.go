package store

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/erdajt/wraith/internal/fsutil"
)

func (s Store) inputPath(directory string) string {
	hash := sha256.Sum256([]byte(canonicalDirectory(directory)))
	return filepath.Join(s.Root, "input", hex.EncodeToString(hash[:])+".json.gz")
}

func (s Store) Inputs(directory string) ([]string, error) {
	f, err := os.Open(s.inputPath(directory))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer z.Close()
	b, err := io.ReadAll(io.LimitReader(z, (4<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(b) > 4<<20 {
		return nil, errors.New("input history exceeds decompression limit")
	}
	var entries []string
	if err := json.Unmarshal(b, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func (s Store) SaveInputs(directory string, entries []string) error {
	return fsutil.Atomic(s.inputPath(directory), func(w io.Writer) error {
		z := gzip.NewWriter(w)
		if err := json.NewEncoder(z).Encode(entries); err != nil {
			z.Close()
			return err
		}
		return z.Close()
	})
}
