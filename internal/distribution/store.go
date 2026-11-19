package distribution

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

type packageStore struct {
	root            *os.Root
	base, objects   *os.File
	directory, name string
}

// openStore pins the configured directory and confines managed paths beneath it.
func openStore(directory, name string) (_ *packageStore, err error) {
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, err
	}
	base, err := os.OpenFile(directory, os.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(descriptorPath(base))
	if err != nil {
		base.Close()
		return nil, err
	}
	store := &packageStore{root: root, base: base, directory: directory, name: name}
	defer func() {
		if err != nil {
			store.close()
		}
	}()

	for _, path := range []string{"objects", "objects/" + name} {
		if err := root.Mkdir(path, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		info, err := root.Lstat(path)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("package store path must be a directory: %s", path)
		}
	}
	store.objects, err = root.OpenFile("objects/"+name, os.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	return store, nil
}

// descriptorPath keeps subprocess paths attached to a parent-owned directory handle.
func descriptorPath(file *os.File) string {
	return fmt.Sprintf("/proc/%d/fd/%d", os.Getpid(), file.Fd())
}

// current rejects replaced managed directories before publication or reuse is reported.
func (s *packageStore) current() error {
	for _, item := range []struct {
		path string
		file *os.File
	}{{s.directory, s.base}, {filepath.Join(s.directory, "objects", s.name), s.objects}} {
		expected, err := item.file.Stat()
		if err != nil {
			return err
		}
		actual, err := os.Lstat(item.path)
		if err != nil {
			return err
		}
		if !actual.IsDir() || !os.SameFile(expected, actual) {
			return errors.New("package store directory changed during installation")
		}
	}
	parent, err := s.root.Lstat("objects")
	if err != nil {
		return err
	}
	if !parent.IsDir() {
		return errors.New("package objects directory was replaced")
	}
	return nil
}

// sync persists the object and complete ancestor chain, including concurrent first installs.
func (s *packageStore) sync() error {
	if err := s.current(); err != nil {
		return err
	}
	if err := s.objects.Sync(); err != nil {
		return err
	}
	for path := filepath.Join(s.directory, "objects"); ; path = filepath.Dir(path) {
		directory, err := os.OpenFile(path, os.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
		if err != nil {
			return err
		}
		if err := errors.Join(directory.Sync(), directory.Close()); err != nil {
			return err
		}
		if path == filepath.Dir(path) {
			break
		}
	}
	return s.current()
}

// close releases the directory handles after every subprocess and publication has finished.
func (s *packageStore) close() {
	if s.objects != nil {
		s.objects.Close()
	}
	s.root.Close()
	s.base.Close()
}
