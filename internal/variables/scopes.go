package variables

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
)

type Scope string

const (
	Conversation Scope = "conversation"
	Directory    Scope = "directory"
	Global       Scope = "global"
)

// Path returns the editable variable file for the requested scope.
func (s Store) Path(directory string, scope Scope) string {
	switch scope {
	case Global:
		return filepath.Join(s.Root, "global.env")
	case Conversation:
		return filepath.Join(s.Root, "conversations", s.Conversation+".env")
	default:
		key := sha256.Sum256([]byte(directory))
		return filepath.Join(s.Root, "directories", hex.EncodeToString(key[:])+".env")
	}
}

// paths resolves the write target and inherited variable files for a scope.
func (s Store) paths(directory string, scope Scope) (string, []string, error) {
	if real, err := filepath.EvalSymlinks(directory); err == nil {
		directory = real
	}
	if !filepath.IsAbs(directory) {
		return "", nil, errors.New("variable directory must be absolute")
	}
	if scope != Conversation && scope != Directory && scope != Global {
		return "", nil, errors.New("variable scope must be conversation, directory or global")
	}

	paths := []string{s.Path(directory, Global), s.Path(directory, Directory)}
	if s.Conversation != "" {
		id, err := hex.DecodeString(s.Conversation)
		if err != nil || len(id) != 12 || hex.EncodeToString(id) != s.Conversation {
			return "", nil, errors.New("invalid variable conversation id")
		}
		paths = append(paths, s.Path(directory, Conversation))
	} else if scope == Conversation {
		return "", nil, errors.New("conversation variables require a conversation id")
	}

	return s.Path(directory, scope), paths, nil
}
