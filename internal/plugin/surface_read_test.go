package plugin

import (
	"strings"
	"testing"
)

func TestSurfaceReadPagesAreBoundedAndDetached(t *testing.T) {
	store := newSurfaceStore("fixture")
	lines := make([]string, 40)
	for i := range lines {
		lines[i] = strings.Repeat("界", 20000)
	}
	store.buffers["buffer"] = &surfaceBufferRecord{SurfaceBuffer: SurfaceBuffer{ID: "buffer", Owner: "config", Revision: 7, Lines: lines}, bytes: 40 * (60000 + 1)}
	page, err := store.BufferLines("buffer", 0, -1)
	if err != nil || page.Revision != 7 || page.Next >= len(lines) || len(page.Lines) == 0 {
		t.Fatalf("unbounded or empty page: %+v %v", page, err)
	}
	size := 0
	for _, line := range page.Lines {
		size += len(line) + 1
	}
	if size > 1<<20 {
		t.Fatalf("page exceeded budget: %d", size)
	}
	page.Lines[0] = "changed"
	buffer, err := store.Buffer("buffer")
	if err != nil || buffer.Lines[0] != lines[0] {
		t.Fatal("page aliased store text")
	}
	buffer.Lines[0] = "changed"
	index := store.Index()
	if len(index.Buffers) != 1 || index.Buffers[0].LineCount != 40 || index.Buffers[0].ByteCount != 40*(60000+1) {
		t.Fatalf("metadata: %+v", index)
	}
	if _, err := store.BufferLines("buffer", 3, 2); err == nil {
		t.Fatal("reversed range accepted")
	}
	if _, err := store.Buffer("missing"); err == nil {
		t.Fatal("missing buffer accepted")
	}
}
