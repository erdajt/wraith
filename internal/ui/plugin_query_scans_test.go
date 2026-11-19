package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/erdajt/wraith/internal/plugin"
)

// TestWorkspaceScanContinuation checks bounded work across empty filtered pages and consumed cursors.
func TestWorkspaceScanContinuation(t *testing.T) {
	root := t.TempDir()
	for index := range 70 {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("%024x.json.gz", index)), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	store := &workspaceScanStore{}
	t.Cleanup(func() { store.reconcile(nil, true) })
	query := plugin.WorkspaceQuery{Owner: "owner", Lease: "lease", Kind: "jobs.list"}
	seen := map[string]bool{}
	examine := func(id string) (any, error) {
		if seen[id] {
			t.Fatalf("duplicate record: %s", id)
		}
		seen[id] = true
		return nil, nil
	}
	page, err := store.page(t.Context(), nil, query, root, "filter", examine)
	if err != nil || page.Examined != 64 || len(page.Items) != 0 || page.Next == "" || page.Complete {
		t.Fatalf("first scan = %+v, %v", page, err)
	}
	query.Options.Cursor = page.Next
	page, err = store.page(t.Context(), nil, query, root, "filter", examine)
	if err != nil || page.Examined != 6 || !page.Complete || page.Next != "" || len(seen) != 70 {
		t.Fatalf("remaining scan = %+v, %v", page, err)
	}
	if _, err := store.page(t.Context(), nil, query, root, "filter", examine); err == nil {
		t.Fatal("consumed cursor accepted")
	}
}

// TestWorkspaceScanInvalidation binds cursors to owners, filters and directory generations.
func TestWorkspaceScanInvalidation(t *testing.T) {
	root := t.TempDir()
	for index := range 2 {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("%024x.json.gz", index)), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	store := &workspaceScanStore{}
	t.Cleanup(func() { store.reconcile(nil, true) })
	query := plugin.WorkspaceQuery{Owner: "owner", Lease: "lease", Kind: "jobs.list", Options: plugin.WorkspaceOptions{Limit: 1}}
	examine := func(id string) (any, error) { return id, nil }
	page, err := store.page(t.Context(), nil, query, root, "filter", examine)
	if err != nil {
		t.Fatal(err)
	}
	query.Options.Cursor = page.Next
	foreign := query
	foreign.Owner = "other"
	if _, err := store.page(t.Context(), nil, foreign, root, "filter", examine); err == nil {
		t.Fatal("foreign cursor accepted")
	}
	if _, err := store.page(t.Context(), nil, query, root, "changed", examine); err == nil {
		t.Fatal("changed filter accepted")
	}
	if err := os.WriteFile(filepath.Join(root, "new.json.gz"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.page(t.Context(), nil, query, root, "filter", examine); err == nil {
		t.Fatal("changed directory generation accepted")
	}
	if len(store.entries) != 0 {
		t.Fatal("invalid scan retained its directory")
	}
}

// TestWorkspaceScanCancellationCannotResurrectClosedIterator covers cancellation while a page is read.
func TestWorkspaceScanCancellationCannotResurrectClosedIterator(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "000000000000000000000000.json.gz"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	store := &workspaceScanStore{}
	ctx, cancel := context.WithCancel(t.Context())
	query := plugin.WorkspaceQuery{Options: plugin.WorkspaceOptions{Limit: 1}}
	_, err := store.page(ctx, nil, query, root, "filter", func(id string) (any, error) {
		cancel()
		store.reconcile(nil, true)
		return id, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if len(store.entries) != 0 {
		t.Fatal("closed iterator was restored")
	}
}
