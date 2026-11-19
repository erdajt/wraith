package distribution

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// Install publishes one verified seed checkout without replacing an existing object.
func Install(ctx context.Context, seeds fs.FS, entry Entry, directory string, api int) (string, error) {
	if err := entry.Validate(api); err != nil {
		return "", err
	}
	directory, err := filepath.Abs(directory)
	if err != nil {
		return "", err
	}
	store, err := openStore(directory, entry.Name)
	if err != nil {
		return "", err
	}
	defer store.close()
	objects := descriptorPath(store.objects)
	destination := filepath.Join(objects, entry.Commit)
	published := filepath.Join(directory, "objects", entry.Name, entry.Commit)
	if _, err := os.Lstat(destination); err == nil {
		if err := verifyCheckout(ctx, destination, entry); err != nil {
			return "", err
		}
		return published, store.sync()
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	data, err := ReadSeed(seeds, entry)
	if err != nil {
		return "", err
	}
	stage, err := os.MkdirTemp(objects, ".pending-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage)
	stageDirectory, err := os.OpenFile(stage, os.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return "", err
	}
	defer stageDirectory.Close()
	artifact := filepath.Join(descriptorPath(stageDirectory), "seed.bundle")
	if err := os.WriteFile(artifact, data, 0o600); err != nil {
		return "", err
	}
	checkout := filepath.Join(descriptorPath(stageDirectory), "checkout")
	if err := os.Mkdir(checkout, 0o700); err != nil {
		return "", err
	}

	for _, arguments := range [][]string{
		{"init", "--quiet", "--template=", "--object-format=" + entry.ObjectFormat, "."},
		{"bundle", "verify", artifact},
		{"fetch", "--quiet", "--no-tags", artifact, "refs/wraith/seed"},
		{"cat-file", "-e", entry.Commit + "^{commit}"},
		{"fsck", "--strict", "--no-reflogs"},
		{"config", "--local", "remote.origin.url", entry.Source},
	} {
		if _, err := runGit(ctx, checkout, arguments...); err != nil {
			return "", err
		}
	}
	if err := verifyTree(ctx, checkout, entry, false); err != nil {
		return "", err
	}
	if _, err := runGit(ctx, checkout, "checkout", "--quiet", "--detach", entry.Commit); err != nil {
		return "", err
	}
	if err := verifyCheckout(ctx, checkout, entry); err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := syncCheckout(ctx, checkout); err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	if err := store.current(); err != nil {
		return "", err
	}
	if err := unix.Renameat2(int(stageDirectory.Fd()), "checkout", int(store.objects.Fd()), entry.Commit, unix.RENAME_NOREPLACE); err != nil {
		if verifyErr := verifyCheckout(ctx, destination, entry); verifyErr != nil {
			return "", fmt.Errorf("publish package: %w; existing object: %v", err, verifyErr)
		}
	}
	if err := store.sync(); err != nil {
		return "", fmt.Errorf("package published but durability is uncertain: %w", err)
	}
	return published, nil
}

// syncCheckout persists staged files and directories before publication.
func syncCheckout(ctx context.Context, directory string) error {
	return filepath.WalkDir(directory, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if !entry.IsDir() && !entry.Type().IsRegular() {
			return errors.New("staged checkout contains a nonregular file")
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		return errors.Join(file.Sync(), file.Close())
	})
}
