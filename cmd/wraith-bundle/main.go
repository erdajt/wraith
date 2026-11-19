package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/erdajt/wraith/internal/distribution"
	"github.com/erdajt/wraith/internal/fsutil"
	"golang.org/x/sys/unix"
)

// main exports a reviewed repository revision into a host distribution directory.
func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run validates source identity before publishing a content-addressed seed and manifest.
func run(ctx context.Context, arguments []string, output io.Writer) error {
	flags := flag.NewFlagSet("wraith-bundle", flag.ContinueOnError)
	flags.SetOutput(output)
	source := flags.String("source", "", "clean repository checkout")
	directory := flags.String("output", "", "distribution assets directory")
	entry := distribution.Entry{Size: 1, SHA256: strings.Repeat("0", 64), Artifact: "seed.bundle"}
	flags.StringVar(&entry.Name, "name", "", "package identity")
	flags.StringVar(&entry.Source, "repository", "", "canonical github url ending in .git")
	flags.StringVar(&entry.Commit, "commit", "", "full commit id")
	flags.StringVar(&entry.ObjectFormat, "object-format", "sha1", "git object format")
	flags.StringVar(&entry.Module, "module", "", "lua entry module")
	flags.StringVar(&entry.License, "license", "LICENSE", "repository license path")
	flags.IntVar(&entry.APIMin, "api-min", 2, "minimum host api")
	flags.IntVar(&entry.APIMax, "api-max", 2, "maximum host api")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *source == "" || *directory == "" || flags.NArg() != 0 {
		return errors.New("source and output are required; positional arguments are not supported")
	}
	entry, bundle, err := distribution.Build(ctx, *source, entry)
	if err != nil {
		return err
	}
	lock, err := lockManifest(ctx, *directory)
	if err != nil {
		return err
	}
	defer lock.Close()
	manifest := distribution.Manifest{Version: 1}
	path := filepath.Join(*directory, "manifest.json")
	data, err := fsutil.ReadLimit(path, 64<<10)
	if err == nil {
		manifest, err = distribution.Parse(data, 2)
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	entry.Artifact = "seeds/" + entry.Name + "-" + entry.SHA256 + ".bundle"
	manifest.Packages = slices.DeleteFunc(manifest.Packages, func(old distribution.Entry) bool { return old.Name == entry.Name })
	manifest.Packages = append(manifest.Packages, entry)
	slices.SortFunc(manifest.Packages, func(a, b distribution.Entry) int { return strings.Compare(a.Name, b.Name) })
	data, err = json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if _, err := distribution.Parse(data, 2); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := fsutil.Bytes(filepath.Join(*directory, filepath.FromSlash(entry.Artifact)), bundle); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := fsutil.Bytes(path, append(data, '\n')); err != nil {
		return err
	}
	_, err = fmt.Fprintf(output, "%s %s %s\n", entry.Name, entry.Commit, entry.SHA256)
	return err
}

// lockManifest serializes source-pin publication across importer processes without locking builds.
func lockManifest(ctx context.Context, directory string) (*os.File, error) {
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(filepath.Join(directory, ".manifest.lock"), os.O_CREATE|os.O_RDWR|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0o600)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		file.Close()
		return nil, errors.New("manifest lock must be a regular file")
	}
	for {
		err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			return file, nil
		}
		if !errors.Is(err, unix.EWOULDBLOCK) {
			file.Close()
			return nil, err
		}
		select {
		case <-ctx.Done():
			file.Close()
			return nil, ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}
