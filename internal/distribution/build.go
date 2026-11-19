package distribution

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Build exports an exact clean repository revision without copying local refs or changing its files.
func Build(ctx context.Context, source string, entry Entry) (Entry, []byte, error) {
	if err := entry.Validate(entry.APIMin); err != nil {
		return Entry{}, nil, err
	}
	source, err := filepath.Abs(source)
	if err != nil {
		return Entry{}, nil, err
	}
	if err := verifyCheckout(ctx, source, entry); err != nil {
		return Entry{}, nil, err
	}
	shallow, err := runGit(ctx, source, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return Entry{}, nil, err
	}
	if shallow != "false\n" {
		return Entry{}, nil, fmt.Errorf("package export requires complete history; fetch the source without a depth limit")
	}
	directory, err := os.MkdirTemp("", "wraith-bundle-")
	if err != nil {
		return Entry{}, nil, err
	}
	defer os.RemoveAll(directory)
	artifact := filepath.Join(directory, "seed.bundle")
	version := "2"
	if entry.ObjectFormat == "sha256" {
		version = "3"
	}
	for _, arguments := range [][]string{
		{"init", "--quiet", "--template=", "--object-format=" + entry.ObjectFormat, "."},
		{"fetch", "--quiet", "--no-tags", source, entry.Commit},
		{"update-ref", "refs/wraith/seed", entry.Commit},
		{"-c", "pack.threads=1", "-c", "pack.window=0", "-c", "pack.depth=0", "bundle", "create", "--version=" + version, artifact, "refs/wraith/seed"},
	} {
		if _, err := runGit(ctx, directory, arguments...); err != nil {
			return Entry{}, nil, err
		}
	}
	file, err := os.Open(artifact)
	if err != nil {
		return Entry{}, nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxSeedSize+1))
	if err != nil {
		return Entry{}, nil, err
	}
	entry.Size, entry.SHA256 = int64(len(data)), fmt.Sprintf("%x", sha256.Sum256(data))
	if err := entry.Validate(entry.APIMin); err != nil {
		return Entry{}, nil, err
	}
	if err := validateBundle(data, entry); err != nil {
		return Entry{}, nil, err
	}
	return entry, data, nil
}
