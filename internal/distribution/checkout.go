package distribution

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// verifyCheckout checks source, revision and local changes before reusing published code.
func verifyCheckout(ctx context.Context, directory string, entry Entry) error {
	for _, path := range []string{directory, filepath.Join(directory, ".git")} {
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("package checkout requires ordinary worktree and git directories")
		}
	}
	for _, check := range []struct {
		arguments []string
		expected  string
	}{
		{[]string{"config", "--local", "--no-includes", "--get-all", "remote.origin.url"}, entry.Source + "\n"},
		{[]string{"rev-parse", "--verify", "--end-of-options", "HEAD^{commit}"}, entry.Commit + "\n"},
		{[]string{"rev-parse", "--show-object-format"}, entry.ObjectFormat + "\n"},
	} {
		output, err := runGit(ctx, directory, check.arguments...)
		if err != nil {
			return err
		}
		if output != check.expected {
			return fmt.Errorf("package checkout %s differs from its manifest; preserve or relocate it before retrying", check.arguments[0])
		}
	}
	return verifyTree(ctx, directory, entry, true)
}

// verifyTree bounds checkout contents and requires package entry and license files.
func verifyTree(ctx context.Context, directory string, entry Entry, worktree bool) error {
	output, err := runGit(ctx, directory, "ls-tree", "-r", "-l", "-z", entry.Commit)
	if err != nil {
		return err
	}
	files := map[string]bool{}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer root.Close()
	var total int64
	for _, record := range strings.Split(strings.TrimSuffix(output, "\x00"), "\x00") {
		metadata, path, ok := strings.Cut(record, "\t")
		fields := strings.Fields(metadata)
		if !ok || len(fields) != 4 || fields[1] != "blob" || (fields[0] != "100644" && fields[0] != "100755") {
			return errors.New("package tree contains unsupported objects, symlinks or submodules")
		}
		size, err := strconv.ParseInt(fields[3], 10, 64)
		if err != nil || size < 0 || size > 8<<20 {
			return errors.New("package file exceeds 8 mib")
		}
		total += size
		files[path] = true
		if total > 64<<20 || len(files) > 4096 {
			return errors.New("package tree exceeds 64 mib or 4096 files")
		}
		if worktree {
			if err := verifyFile(ctx, root, path, size, fields[2], fields[0], entry.ObjectFormat); err != nil {
				return err
			}
		}
	}
	module := "lua/" + strings.ReplaceAll(entry.Module, ".", "/")
	if !files[entry.License] || (!files[module+".lua"] && !files[module+"/init.lua"]) {
		return errors.New("package tree is missing its declared license or entry module")
	}
	if worktree {
		return fs.WalkDir(root.FS(), ".", func(path string, item fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if path == ".git" {
				return fs.SkipDir
			}
			if item.IsDir() {
				return nil
			}
			if !files[path] || !item.Type().IsRegular() {
				return fmt.Errorf("package checkout has an unexpected file: %s", path)
			}
			return ctx.Err()
		})
	}
	return nil
}

// verifyFile compares raw bytes with the git object without running filters or trusting index flags.
func verifyFile(ctx context.Context, root *os.Root, path string, size int64, object, mode, format string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	file, err := root.OpenFile(path, os.O_RDONLY|unix.O_NONBLOCK|unix.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() != size || (info.Mode()&0o111 != 0) != (mode == "100755") {
		return fmt.Errorf("package file type, size or mode changed: %s", path)
	}
	var digest hash.Hash = sha1.New()
	if format == "sha256" {
		digest = sha256.New()
	}
	fmt.Fprintf(digest, "blob %d\x00", size)
	n, err := io.Copy(digest, io.LimitReader(file, size+1))
	if err != nil {
		return err
	}
	if n != size || hex.EncodeToString(digest.Sum(nil)) != object {
		return fmt.Errorf("package source changed: %s; preserve or relocate the checkout before retrying", path)
	}
	return nil
}
