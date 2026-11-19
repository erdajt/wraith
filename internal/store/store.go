package store

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/erdajt/wraith/internal/fsutil"
	"github.com/erdajt/wraith/internal/session"
	"golang.org/x/sys/unix"
)

const maxSession = 64 << 20

var validID = regexp.MustCompile(`^[a-f0-9]{24}$`)
var validArchive = regexp.MustCompile(`^[a-f0-9]{24}-[a-f0-9]{24}\.json\.gz$`)

type Store struct{ Root string }

// Lock takes an exclusive data-directory lock and returns its release function.
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

	return func() {
		_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
		_ = f.Close()
	}, nil
}

// Save validates the session id and saves a compressed session with an updated timestamp.
func (s Store) Save(v session.Session) error {
	if !validID.MatchString(v.ID) {
		return errors.New("invalid session id")
	}

	v.Updated = time.Now().UTC()
	return compressed(filepath.Join(s.Root, "sessions", v.ID+".json.gz"), v)
}

// Load loads a session and verifies its id against the requested filename.
func (s Store) Load(id string) (session.Session, error) {
	return s.LoadContext(context.Background(), id)
}

// LoadContext bounds and cancels query reads without changing the saved session format.
func (s Store) LoadContext(ctx context.Context, id string) (session.Session, error) {
	return s.LoadBounded(ctx, id, maxSession)
}

// LoadBounded applies a smaller source budget to catalog queries.
func (s Store) LoadBounded(ctx context.Context, id string, limit int64) (session.Session, error) {
	if limit < 1 || limit > maxSession {
		return session.Session{}, errors.New("invalid session source limit")
	}
	if !validID.MatchString(id) {
		return session.Session{}, errors.New("invalid session id")
	}

	v, err := readContext(ctx, filepath.Join(s.Root, "sessions", id+".json.gz"), limit)
	if err == nil && v.ID != id {
		return v, errors.New("session id does not match file")
	}
	return v, err
}

// Archive writes a compressed snapshot under a new archive name.
func (s Store) Archive(v session.Session) (string, error) {
	if !validID.MatchString(v.ID) {
		return "", errors.New("invalid session id")
	}

	name := v.ID + "-" + session.ID() + ".json.gz"
	return name, compressed(filepath.Join(s.Root, "archives", name), v)
}

// ReadArchive validates an archive name before loading its session.
func (s Store) ReadArchive(name string) (session.Session, error) {
	if !validArchive.MatchString(name) {
		return session.Session{}, errors.New("invalid archive name")
	}

	return read(filepath.Join(s.Root, "archives", name))
}

// List returns readable sessions by update time and joins errors for unreadable files.
func (s Store) List() ([]session.Session, error) {
	paths, err := filepath.Glob(filepath.Join(s.Root, "sessions", "*.json.gz"))
	if err != nil {
		return nil, err
	}

	var out []session.Session
	var problems []error
	for _, p := range paths {
		v, e := s.Load(strings.TrimSuffix(filepath.Base(p), ".json.gz"))
		if e != nil {
			problems = append(problems, fmt.Errorf("%s: %w", filepath.Base(p), e))
			continue
		}
		out = append(out, v)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Updated.After(out[j].Updated) })
	return out, errors.Join(problems...)
}

// compressed atomically writes a session as compressed json.
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

// read loads a session with a decompression limit and validates its format.
func read(path string) (session.Session, error) {
	return readContext(context.Background(), path, maxSession)
}

// readContext rejects special files and checks cancellation between decompression reads.
func readContext(ctx context.Context, path string, limit int64) (session.Session, error) {
	var v session.Session
	f, err := os.OpenFile(path, os.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return v, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return v, err
	}
	if !info.Mode().IsRegular() {
		return v, errors.New("session is not a regular file")
	}

	z, err := gzip.NewReader(sessionReader{ctx, f})
	if err != nil {
		return v, err
	}
	defer z.Close()

	b, err := io.ReadAll(io.LimitReader(sessionReader{ctx, z}, limit+1))
	if err != nil {
		return v, err
	}
	if int64(len(b)) > limit {
		return v, errors.New("session exceeds decompression limit")
	}

	if err = json.Unmarshal(b, &v); err != nil {
		return v, err
	}
	if v.Version != 1 || !validID.MatchString(v.ID) {
		return v, errors.New("invalid session format")
	}
	return v, ctx.Err()
}

type sessionReader struct {
	context context.Context
	reader  io.Reader
}

// Read checks cancellation before reading the next bounded source chunk.
func (r sessionReader) Read(p []byte) (int, error) {
	if err := r.context.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}
