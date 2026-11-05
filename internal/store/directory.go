package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/erdajt/wraith/internal/session"
)

func canonicalDirectory(path string) string {
	if path == "" {
		return ""
	}
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	return filepath.Clean(path)
}

func (s Store) ListDirectory(directory string) ([]session.Session, error) {
	all, err := s.List()
	if err != nil {
		return nil, err
	}
	var found []session.Session
	for _, chat := range all {
		if chat.Directory != "" && canonicalDirectory(chat.Directory) == canonicalDirectory(directory) {
			found = append(found, chat)
		}
	}
	return found, nil
}

func (s Store) LoadDirectory(id, directory string) (session.Session, error) {
	chat, err := s.Load(id)
	if err != nil {
		return session.Session{}, errors.New("invalid session id")
	}
	if chat.Directory == "" {
		return session.Session{}, errors.New("session has no recorded original directory; export it to recover the conversation")
	}
	if canonicalDirectory(chat.Directory) != canonicalDirectory(directory) {
		path := chat.Directory
		if home, err := os.UserHomeDir(); err == nil {
			if path == home {
				path = "~"
			} else if strings.HasPrefix(path, home+string(filepath.Separator)) {
				path = "~" + strings.TrimPrefix(path, home)
			}
		}
		return session.Session{}, fmt.Errorf("you are in a different directory; original conversation occurred at: %s", path)
	}
	return chat, nil
}
