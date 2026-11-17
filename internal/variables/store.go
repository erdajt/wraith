package variables

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/erdajt/wraith/internal/fsutil"
	"golang.org/x/sys/unix"
)

type Store struct {
	Root         string
	Conversation string
}

type State struct {
	Text     string
	Revision string
	Entries  []Entry
	Snapshot Snapshot
}

type journal struct {
	Accepted map[string][]Entry `json:"accepted"`
	Retained []Entry            `json:"retained"`
}

func (s Store) Load(directory string, scope Scope) (State, error) {
	return s.transact(directory, scope, "", nil)
}

func (s Store) Save(directory string, scope Scope, revision, text string) (State, error) {
	return s.transact(directory, scope, revision, &text)
}

func (s Store) Edit(directory string, scope Scope) (State, error) {
	selected, _, err := s.paths(directory, scope)
	if err != nil {
		return State{}, err
	}
	text, err := fsutil.ReadLimit(selected, FileLimit)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return State{}, err
	}
	hash := sha256.Sum256(text)
	return State{Text: string(text), Revision: hex.EncodeToString(hash[:])}, nil
}

func (s Store) transact(directory string, scope Scope, revision string, replacement *string) (State, error) {
	var state State
	selected, paths, err := s.paths(directory, scope)
	if err != nil {
		return state, err
	}
	if err := os.MkdirAll(s.Root, 0700); err != nil {
		return state, err
	}
	lock, err := os.OpenFile(filepath.Join(s.Root, ".lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return state, err
	}
	defer lock.Close()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		return state, err
	}
	defer unix.Flock(int(lock.Fd()), unix.LOCK_UN)
	j := journal{Accepted: map[string][]Entry{}}
	path := filepath.Join(s.Root, "state.json")
	data, err := fsutil.ReadLimit(path, 8<<20)
	if err == nil {
		if json.Unmarshal(data, &j) != nil || j.Accepted == nil {
			return state, errors.New("private variable journal is corrupt; restore its backup before sending prompts")
		}
	} else if errors.Is(err, os.ErrNotExist) {
		if _, e := os.Stat(filepath.Join(s.Root, ".initialized")); e == nil {
			return state, errors.New("private variable journal is missing; restore its backup before sending prompts")
		}
	} else {
		return state, err
	}
	active := map[string]Entry{}
	for _, scope := range paths {
		text, err := fsutil.ReadLimit(scope, FileLimit)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return state, err
		}
		entries, err := Parse(string(text))
		if err != nil && (scope != selected || replacement == nil) {
			return state, err
		}
		if scope == selected {
			hash := sha256.Sum256(text)
			state.Text, state.Revision = string(text), hex.EncodeToString(hash[:])
			if replacement != nil {
				if revision != state.Revision {
					return state, errors.New("variable file changed; reconcile the retained edit before saving")
				}
				entries, err = Parse(*replacement)
				if err != nil {
					return state, err
				}
			}
			state.Entries = entries
		}
		j.Retained = append(j.Retained, j.Accepted[scope]...)
		j.Accepted[scope] = entries
		for _, entry := range entries {
			active[entry.Name] = entry
		}
	}
	seen := map[string]bool{}
	retained := make([]Entry, 0, len(j.Retained))
	for _, entry := range j.Retained {
		key := entry.Name + "\x00" + entry.Value
		if !seen[key] {
			retained = append(retained, Entry{Name: entry.Name, Value: entry.Value})
			seen[key] = true
		}
	}
	j.Retained = retained
	var entries []Entry
	for _, entry := range active {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	var scopes []string
	for scope := range j.Accepted {
		scopes = append(scopes, scope)
	}
	sort.Strings(scopes)
	redactions := append([]Entry(nil), retained...)
	for _, scope := range scopes {
		redactions = append(redactions, j.Accepted[scope]...)
	}
	state.Snapshot = NewSnapshot(entries, redactions)
	if _, err := state.Snapshot.Inventory(); err != nil {
		return state, err
	}
	encoded, err := json.Marshal(j)
	if err != nil {
		return state, err
	}
	if len(encoded) > 8<<20 {
		return state, errors.New("private variable journal exceeds 8 mib; changes were not applied")
	}
	if err := fsutil.Bytes(path, encoded); err != nil {
		return state, fmt.Errorf("save private variable journal: %w", err)
	}
	if err := fsutil.Bytes(filepath.Join(s.Root, ".initialized"), []byte("1\n")); err != nil {
		return state, err
	}
	if replacement != nil {
		if err := fsutil.Bytes(selected, []byte(*replacement)); err != nil {
			return state, err
		}
		state.Text = *replacement
		hash := sha256.Sum256([]byte(*replacement))
		state.Revision = hex.EncodeToString(hash[:])
	}
	return state, nil
}
