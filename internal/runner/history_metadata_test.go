package runner

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// TestMetadataCache checks source binding, repair and independence from committed payloads.
func TestMetadataCache(t *testing.T) {
	history := History{Root: t.TempDir()}
	record := Record{ID: "0123456789abcdef01234567", Command: "printf first", Output: "private output", Tail: "private tail"}
	if err := history.Save(record); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(history.Root, "metadata", record.ID+".json")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(original), "private output") || strings.Contains(string(original), "private tail") {
		t.Fatal("metadata retained captured output")
	}
	metadata, err := history.Metadata(t.Context(), record.ID)
	if err != nil || metadata.Command != record.Command || metadata.Output != "" || metadata.Tail != "" {
		t.Fatalf("metadata = %#v, %v", metadata, err)
	}

	record.Command = "printf second"
	if err := history.Save(record); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	metadata, err = history.Metadata(t.Context(), record.ID)
	if err != nil || metadata.Command != record.Command {
		t.Fatalf("stale metadata accepted: %#v, %v", metadata, err)
	}
	if err := os.WriteFile(path, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := history.Metadata(t.Context(), record.ID); err != nil {
		t.Fatalf("corrupt cache was not repaired: %v", err)
	}

	if err := os.RemoveAll(filepath.Dir(path)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Dir(path), []byte("blocks cache writes"), 0o600); err != nil {
		t.Fatal(err)
	}
	record.Command = "printf committed"
	if err := history.Save(record); err != nil {
		t.Fatalf("optional cache write failed committed save: %v", err)
	}
	loaded, err := history.Load(record.ID)
	if err != nil || loaded.Command != record.Command {
		t.Fatalf("payload = %#v, %v", loaded, err)
	}
}

// TestMetadataLargeSource verifies large new records list without decompressing their output.
func TestMetadataLargeSource(t *testing.T) {
	history := History{Root: t.TempDir()}
	record := Record{ID: "0123456789abcdef01234567", Command: "large", Output: strings.Repeat("x", MetadataSourceLimit+1)}
	if err := history.Save(record); err != nil {
		t.Fatal(err)
	}
	metadata, err := history.Metadata(t.Context(), record.ID)
	if err != nil || metadata.Command != "large" {
		t.Fatalf("large cached metadata = %#v, %v", metadata, err)
	}
	if err := os.Remove(filepath.Join(history.Root, "metadata", record.ID+".json")); err != nil {
		t.Fatal(err)
	}
	if _, err := history.Metadata(t.Context(), record.ID); !errors.Is(err, ErrHistoryLimit) {
		t.Fatalf("legacy budget error = %v", err)
	}
}

// TestLoadBounded checks exact source budgets and cancellation before allocation.
func TestLoadBounded(t *testing.T) {
	history := History{Root: t.TempDir()}
	record := Record{ID: "0123456789abcdef01234567", Command: "printf ok", Output: "ok"}
	if err := history.Save(record); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := history.LoadBounded(t.Context(), record.ID, int64(len(data))); !errors.Is(err, ErrHistoryLimit) {
		t.Fatalf("source without encoder newline should exceed limit: %v", err)
	}
	if _, err := history.LoadBounded(t.Context(), record.ID, int64(len(data)+1)); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := history.LoadBounded(ctx, record.ID, 1024); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled load = %v", err)
	}
}

// TestHistorySpecialFiles rejects blocking sources and repairs nonregular cache entries.
func TestHistorySpecialFiles(t *testing.T) {
	history := History{Root: t.TempDir()}
	id := "0123456789abcdef01234567"
	path := filepath.Join(history.Root, id+".json.gz")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := history.LoadBounded(t.Context(), id, 1024); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("fifo source = %v", err)
	}
	if _, err := history.Metadata(t.Context(), id); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("fifo metadata source = %v", err)
	}
	if err := history.Save(Record{ID: id, Command: "printf safe"}); err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(history.Root, "metadata", id+".json")
	if err := os.Remove(cache); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(cache, 0o600); err != nil {
		t.Fatal(err)
	}
	record, err := history.Metadata(t.Context(), id)
	if err != nil || record.Command != "printf safe" {
		t.Fatalf("fifo cache fallback = %#v, %v", record, err)
	}
}
