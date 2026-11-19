package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/erdajt/wraith/internal/session"
)

// TestSessionQueryReads rejects special files and preserves cancellation on stored snapshots.
func TestSessionQueryReads(t *testing.T) {
	store := Store{Root: t.TempDir()}
	value := session.Session{Version: 1, ID: "0123456789abcdef01234567", Messages: []session.Message{{Role: "user", Content: "saved"}}}
	if err := store.Save(value); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := store.LoadContext(ctx, value.ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled session read: %v", err)
	}
	loaded, err := store.LoadContext(t.Context(), value.ID)
	if err != nil || loaded.Messages[0].Content != "saved" {
		t.Fatalf("session read = %#v, %v", loaded, err)
	}
	path := filepath.Join(store.Root, "sessions", value.ID+".json.gz")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LoadContext(t.Context(), value.ID); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("fifo session read = %v", err)
	}
}
