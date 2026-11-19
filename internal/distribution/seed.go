package distribution

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
)

// ReadSeed verifies an artifact from an immutable, confined filesystem before git reads it.
func ReadSeed(root fs.FS, entry Entry) ([]byte, error) {
	if err := entry.Validate(entry.APIMin); err != nil {
		return nil, err
	}
	parts := strings.Split(entry.Artifact, "/")
	for index, part := range parts {
		parent := "."
		if index > 0 {
			parent = strings.Join(parts[:index], "/")
		}
		entries, err := fs.ReadDir(root, parent)
		if err != nil {
			return nil, fmt.Errorf("seed directory: %w", err)
		}
		for _, candidate := range entries {
			if candidate.Name() != part {
				continue
			}
			if index < len(parts)-1 && !candidate.IsDir() {
				return nil, errors.New("seed parent must be a directory")
			}
			if index == len(parts)-1 && !candidate.Type().IsRegular() {
				return nil, errors.New("seed artifact must be a regular file")
			}
		}
	}
	file, err := root.Open(entry.Artifact)
	if err != nil {
		return nil, fmt.Errorf("open seed: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat seed: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() != entry.Size {
		return nil, errors.New("seed must be a regular file matching the declared size")
	}
	data, err := io.ReadAll(io.LimitReader(file, entry.Size+1))
	if err != nil {
		return nil, fmt.Errorf("read seed: %w", err)
	}
	if int64(len(data)) != entry.Size {
		return nil, errors.New("seed size changed while reading")
	}
	digest := sha256.Sum256(data)
	if hex.EncodeToString(digest[:]) != entry.SHA256 {
		return nil, errors.New("seed checksum mismatch")
	}
	if err := validateBundle(data, entry); err != nil {
		return nil, err
	}
	return data, nil
}

// validateBundle requires a self-contained bundle advertising exactly the pinned commit.
func validateBundle(data []byte, entry Entry) error {
	end := bytes.Index(data, []byte("\n\n"))
	if end < 0 || end > 64<<10 || len(data) < end+6 || string(data[end+2:end+6]) != "PACK" {
		return errors.New("invalid git bundle header or pack")
	}
	lines := strings.Split(string(data[:end]), "\n")
	format := "sha1"
	switch lines[0] {
	case "# v2 git bundle":
		lines = lines[1:]
	case "# v3 git bundle":
		lines = lines[1:]
		if len(lines) > 0 && strings.HasPrefix(lines[0], "@object-format=") {
			format = strings.TrimPrefix(lines[0], "@object-format=")
			lines = lines[1:]
		}
	default:
		return errors.New("unsupported git bundle version")
	}
	if format != entry.ObjectFormat || len(lines) != 1 || lines[0] != entry.Commit+" refs/wraith/seed" {
		return errors.New("seed must advertise only the pinned commit without prerequisites")
	}
	return nil
}
