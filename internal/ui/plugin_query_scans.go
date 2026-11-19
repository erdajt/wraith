package ui

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/erdajt/wraith/internal/plugin"
	"github.com/erdajt/wraith/internal/session"
)

type workspaceScan struct {
	file       *os.File
	generation os.FileInfo
	runtime    *plugin.Runtime
	owner      string
	lease      string
	filter     string
	expires    time.Time
	busy       bool
}

type workspaceScanStore struct {
	mu      sync.Mutex
	entries map[string]*workspaceScan
	closed  bool
}

type workspaceScanPage struct {
	Items    []any            `json:"items"`
	Errors   []map[string]any `json:"errors"`
	Next     string           `json:"next"`
	Complete bool             `json:"complete"`
	Examined int              `json:"examined"`
}

// page retains directory iterators so continuation never rescans an unbounded prefix.
func (s *workspaceScanStore) page(ctx context.Context, runtime *plugin.Runtime, query plugin.WorkspaceQuery, directory, filter string, budget int, examine func(string) (any, error)) (workspaceScanPage, error) {
	page := workspaceScanPage{Items: []any{}, Errors: []map[string]any{}}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return page, errors.New("workspace scans are closed")
	}
	if s.entries == nil {
		s.entries = map[string]*workspaceScan{}
	}
	for token, scan := range s.entries {
		if !scan.busy && time.Now().After(scan.expires) {
			_ = scan.file.Close()
			delete(s.entries, token)
		}
	}
	token := query.Options.Cursor
	scan := s.entries[token]
	if token != "" {
		if scan == nil || scan.runtime != runtime || scan.owner != query.Owner || scan.lease != query.Lease || scan.filter != filter || scan.busy {
			s.mu.Unlock()
			return page, errors.New("stale, busy or expired workspace scan cursor")
		}
	} else {
		if len(s.entries) >= 16 {
			s.mu.Unlock()
			return page, errors.New("workspace scan capacity reached")
		}
		s.mu.Unlock()
		file, err := os.OpenFile(directory, os.O_RDONLY|syscall.O_NONBLOCK, 0)
		if errors.Is(err, os.ErrNotExist) {
			page.Complete = true
			return page, nil
		}
		if err != nil {
			return page, err
		}
		info, err := file.Stat()
		if err != nil || !info.IsDir() {
			_ = file.Close()
			return page, errors.New("workspace records require a directory")
		}
		s.mu.Lock()
		if s.closed || len(s.entries) >= 16 {
			s.mu.Unlock()
			_ = file.Close()
			return page, errors.New("workspace scan capacity reached or closed")
		}
		scan = &workspaceScan{file: file, generation: info, runtime: runtime, owner: query.Owner, lease: query.Lease, filter: filter}
		token = session.ID()
		s.entries[token] = scan
	}
	scan.busy = true
	s.mu.Unlock()

	keep := false
	defer func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		live := s.entries[token] == scan
		delete(s.entries, token)
		if keep && live && !s.closed {
			scan.busy = false
			scan.expires = time.Now().Add(2 * time.Minute)
			s.entries[page.Next] = scan
		} else {
			_ = scan.file.Close()
		}
	}()
	generationMatches := func() bool {
		info, err := os.Stat(directory)
		return err == nil && os.SameFile(scan.generation, info) && info.ModTime() == scan.generation.ModTime()
	}
	if !generationMatches() {
		return page, errors.New("workspace records changed; restart scan")
	}
	limit := query.Options.Limit
	if limit == 0 {
		limit = 40
	}
	for page.Examined < min(64, budget) && len(page.Items) < limit {
		if err := ctx.Err(); err != nil {
			return page, err
		}
		entries, err := scan.file.ReadDir(1)
		if errors.Is(err, io.EOF) {
			page.Complete = true
			break
		}
		if err != nil {
			return page, err
		}
		page.Examined++
		name := entries[0].Name()
		if entries[0].IsDir() || !strings.HasSuffix(name, ".json.gz") {
			continue
		}
		id := strings.TrimSuffix(name, ".json.gz")
		item, err := examine(id)
		if err != nil {
			page.Errors = append(page.Errors, map[string]any{"id": id, "message": err.Error()})
		} else if item != nil {
			page.Items = append(page.Items, item)
		}
	}
	if err := ctx.Err(); err != nil {
		return workspaceScanPage{}, err
	}
	if !generationMatches() {
		return workspaceScanPage{}, errors.New("workspace records changed; restart scan")
	}
	if !page.Complete {
		page.Next = session.ID()
		keep = true
	}
	return page, nil
}

// reconcile closes expired or disposed-owner cursors, including outstanding page reads.
func (s *workspaceScanStore) reconcile(runtime *plugin.Runtime, closeAll bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if closeAll {
		s.closed = true
	}
	for token, scan := range s.entries {
		if closeAll || scan.runtime != runtime || !runtime.Surfaces().OwnerValid(scan.owner, scan.lease) || !scan.busy && time.Now().After(scan.expires) {
			_ = scan.file.Close()
			delete(s.entries, token)
		}
	}
}
