package store

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/erdajt/wraith/internal/fsutil"
	"github.com/erdajt/wraith/internal/session"
	"golang.org/x/sys/unix"
)

const maxSession = 64 << 20

var validID = regexp.MustCompile(`^[a-f0-9]{24}$`)
var validArchive = regexp.MustCompile(`^[a-f0-9]{24}-[a-f0-9]{24}\.json\.gz$`)

type Store struct{ Root string }

func (s Store) Lock() (func(), error) {
	if err := os.MkdirAll(s.Root, 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(s.Root, ".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = f.Close()
		return nil, errors.New("another wraith process owns this data directory")
	}
	return func() { _ = unix.Flock(int(f.Fd()), unix.LOCK_UN); _ = f.Close() }, nil
}

func (s Store) Save(v session.Session) error {
	if !validID.MatchString(v.ID) {
		return errors.New("invalid session id")
	}
	v.Updated = time.Now().UTC()
	return compressed(filepath.Join(s.Root, "sessions", v.ID+".json.gz"), v)
}

func (s Store) Load(id string) (session.Session, error) {
	if !validID.MatchString(id) {
		return session.Session{}, errors.New("invalid session id")
	}
	v, err := read(filepath.Join(s.Root, "sessions", id+".json.gz"))
	if err == nil && v.ID != id {
		return v, errors.New("session id does not match file")
	}
	return v, err
}

func (s Store) Archive(v session.Session) (string, error) {
	if !validID.MatchString(v.ID) {
		return "", errors.New("invalid session id")
	}
	name := v.ID + "-" + session.ID() + ".json.gz"
	return name, compressed(filepath.Join(s.Root, "archives", name), v)
}

func (s Store) ReadArchive(name string) (session.Session, error) {
	if !validArchive.MatchString(name) {
		return session.Session{}, errors.New("invalid archive name")
	}
	return read(filepath.Join(s.Root, "archives", name))
}

func (s Store) List() ([]session.Session, error) {
	paths, err := filepath.Glob(filepath.Join(s.Root, "sessions", "*.json.gz"))
	if err != nil {
		return nil, err
	}
	var out []session.Session
	for _, p := range paths {
		v, e := read(p)
		if e != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Base(p), e)
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Updated.After(out[j].Updated) })
	return out, nil
}

func compressed(path string, v session.Session) error {
	return fsutil.Atomic(path, func(w io.Writer) error {
		z := gzip.NewWriter(w)
		if err := json.NewEncoder(z).Encode(v); err != nil {
			_ = z.Close()
			return err
		}
		return z.Close()
	})
}

func read(path string) (session.Session, error) {
	var v session.Session
	f, err := os.Open(path)
	if err != nil {
		return v, err
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		return v, err
	}
	defer z.Close()
	b, err := io.ReadAll(io.LimitReader(z, maxSession+1))
	if err != nil {
		return v, err
	}
	if len(b) > maxSession {
		return v, errors.New("session exceeds decompression limit")
	}
	if err = json.Unmarshal(b, &v); err != nil {
		return v, err
	}
	if v.Version != 1 || !validID.MatchString(v.ID) {
		return v, errors.New("invalid session format")
	}
	return v, nil
}
