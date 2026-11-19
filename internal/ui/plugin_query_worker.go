package ui

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/erdajt/wraith/internal/plugin"
	"github.com/erdajt/wraith/internal/runner"
	"github.com/erdajt/wraith/internal/session"
	"github.com/erdajt/wraith/internal/store"
)

type workspaceQueryInput struct {
	query     plugin.WorkspaceQuery
	runtime   *plugin.Runtime
	scans     *workspaceScanStore
	store     store.Store
	history   runner.History
	session   session.Session
	jobs      []runner.Job
	directory string
	data      any
}

// run retains a source slot through decoding, matching and bounded projection.
func (input workspaceQueryInput) run(ctx context.Context) (any, error) {
	if input.data != nil {
		return input.read(ctx)
	}
	var result any
	err := input.scans.readSource(ctx, func() error {
		var err error
		result, err = input.read(ctx)
		return err
	})
	return result, err
}

// read reads detached state and bounded disk records without retaining the application or Lua VM.
func (input workspaceQueryInput) read(ctx context.Context) (any, error) {
	query, options := input.query, input.query.Options
	if err := query.Validate(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if input.data != nil {
		return input.data, nil
	}
	directory := input.directory
	if options.Directory != "" {
		var err error
		directory, err = filepath.Abs(options.Directory)
		if err == nil {
			directory, err = filepath.EvalSymlinks(directory)
		}
		if err != nil {
			return nil, err
		}
	}
	if strings.HasSuffix(query.Kind, ".list") {
		return input.list(ctx, directory)
	}
	if strings.HasPrefix(query.Kind, "jobs.") {
		return input.job(ctx, directory)
	}

	value := input.session
	if options.ID != "" && options.ID != value.ID {
		var err error
		value, err = input.store.LoadContext(ctx, options.ID)
		if err != nil {
			return nil, err
		}
	}
	if !store.SameDirectory(value.Directory, directory) {
		return nil, fmt.Errorf("wrong_directory: original conversation occurred at %s", value.Directory)
	}
	revision := workspaceSessionRevision(value)
	if options.Revision != "" && options.Revision != revision {
		return nil, errors.New("stale_revision: conversation changed")
	}
	if query.Kind == "sessions.get" {
		return workspaceSessionMetadata(value), nil
	}
	if query.Kind == "sessions.message" {
		if options.Index >= len(value.Messages) {
			return nil, errors.New("message index is outside conversation")
		}
		text := value.Messages[options.Index].Content
		if options.Field == "thinking" {
			text = value.Messages[options.Index].Thinking
		}
		if !utf8.ValidString(text) {
			return nil, errors.New("message source contains invalid utf-8")
		}
		if options.From > int64(len(text)) || options.From < int64(len(text)) && !utf8.RuneStart(text[options.From]) {
			return nil, errors.New("message offset is not a valid utf-8 boundary")
		}
		limit := options.Limit
		if limit == 0 {
			limit = 16 << 10
		}
		chunk := workspacePreview(text[options.From:], limit)
		if chunk == "" && options.From < int64(len(text)) {
			return nil, errors.New("message limit is too small for the next rune")
		}
		next := options.From + int64(len(chunk))
		return map[string]any{"revision": revision, "text": chunk, "next": next, "complete": next == int64(len(text))}, nil
	}
	if options.From > int64(len(value.Messages)) {
		return nil, errors.New("message offset is outside conversation")
	}
	limit := options.Limit
	if limit == 0 {
		limit = 40
	}
	end := min(len(value.Messages), int(options.From)+limit)
	items := make([]any, 0, end-int(options.From))
	for index := int(options.From); index < end; index++ {
		message := value.Messages[index]
		identity := workspaceSessionRevision(session.Session{Messages: []session.Message{message}})
		items = append(items, map[string]any{
			"index": index, "id": fmt.Sprintf("%s:%d:%s", value.ID, index, identity), "role": message.Role,
			"preview": workspacePreview(message.Content, 1024), "content_bytes": len(message.Content),
			"thinking_bytes": len(message.Thinking), "edited": message.Edited, "interrupted": message.Interrupted,
		})
	}
	return map[string]any{"items": items, "revision": revision, "next": end, "complete": end == len(value.Messages)}, nil
}

// job resolves live records before disk and keeps local reads separate from model evidence.
func (input workspaceQueryInput) job(ctx context.Context, directory string) (any, error) {
	options := input.query.Options
	var record runner.Record
	for _, job := range input.jobs {
		if job.Record.ID == options.ID {
			record = job.Record
			break
		}
	}
	if record.ID == "" {
		var err error
		if input.query.Kind == "jobs.get" {
			record, err = input.history.Metadata(ctx, options.ID)
		} else {
			record, err = input.history.LoadBounded(ctx, options.ID, 64<<20)
		}
		if err != nil {
			return nil, err
		}
	}
	if record.Session != input.session.ID && !store.SameDirectory(record.Directory, directory) {
		return nil, fmt.Errorf("wrong_directory: command ran at %s", record.Directory)
	}
	metadata := workspaceJobMetadata(record)
	if input.query.Kind == "jobs.get" {
		return metadata, nil
	}
	if options.Revision != "" && options.Revision != metadata["revision"] {
		return nil, errors.New("stale_revision: command output changed")
	}
	page, err := runner.ReadLocalOutput(record, runner.Query{
		From: options.From, Pattern: options.Pattern, Regex: options.Regex, IgnoreCase: options.IgnoreCase,
		Before: options.Before, After: options.After, Limit: options.Limit,
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"revision": metadata["revision"], "page": page, "record": metadata}, ctx.Err()
}

// list scans bounded source pages and keeps active state separate from persisted ordering.
func (input workspaceQueryInput) list(ctx context.Context, directory string) (any, error) {
	query := input.query
	root, budget := filepath.Join(input.store.Root, "sessions"), 4
	sessionID := query.Options.Session
	if sessionID == "" && query.Options.Directory == "" {
		sessionID = input.session.ID
	}
	if query.Kind == "jobs.list" {
		root, budget = input.history.Root, 4
	}
	filter := query.Kind + "\x00" + root + "\x00" + directory + "\x00" + sessionID
	page, err := input.scans.page(ctx, input.runtime, query, root, filter, budget, func(id string) (any, error) {
		if query.Kind == "sessions.list" {
			value, err := input.store.LoadBounded(ctx, id, 16<<20)
			if err != nil || !store.SameDirectory(value.Directory, directory) {
				return nil, err
			}
			return workspaceSessionMetadata(value), nil
		}
		value, err := input.history.Metadata(ctx, id)
		if err != nil || sessionID != "" && value.Session != sessionID || sessionID == "" && !store.SameDirectory(value.Directory, directory) {
			return nil, err
		}
		return workspaceJobMetadata(value), nil
	})
	if err != nil {
		return nil, err
	}
	key := "updated"
	if query.Kind == "jobs.list" {
		key = "started"
	}
	sort.SliceStable(page.Items, func(i, j int) bool {
		return fmt.Sprint(page.Items[i].(map[string]any)[key]) > fmt.Sprint(page.Items[j].(map[string]any)[key])
	})
	active := []any{}
	if query.Options.Cursor == "" {
		if query.Kind == "sessions.list" && store.SameDirectory(input.session.Directory, directory) {
			active = append(active, workspaceSessionMetadata(input.session))
		}
		if query.Kind == "jobs.list" {
			for _, job := range input.jobs {
				if sessionID != "" && job.Record.Session == sessionID || sessionID == "" && store.SameDirectory(job.Record.Directory, directory) {
					active = append(active, workspaceJobMetadata(job.Record))
				}
			}
		}
	}
	return map[string]any{"page": page, "active": active}, nil
}
