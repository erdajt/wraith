package plugin

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestAsyncFileWriteStartsOnlyAfterActivation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "created")
	r := openTestRuntime(t, fmt.Sprintf(`wraith.fs.write_async(%q, string.char(255,0,195,169), function(ctx)
 assert(ctx.data.state == "completed" and ctx.data.result.bytes == 4)
 wraith.notify("written")
end)`, path))
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("unaccepted write changed the filesystem: %v", err)
	}
	m := r.Manifest()
	if err := r.Activate(m.Resources); err != nil {
		t.Fatal(err)
	}
	event := receiveResource(t, r)
	pending, err := r.SubmitResource(event, nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	got := receiveResult(t, pending)
	if got.Err != nil || len(got.Effects) != 1 || got.Effects[0].Argument != "written" {
		t.Fatalf("write callback: %+v", got)
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, []byte{255, 0, 195, 169}) {
		t.Fatalf("written bytes: %v, %v", data, err)
	}
}

func TestAsyncFileReadPreservesBinaryCallbackData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bytes")
	if err := os.WriteFile(path, []byte{255, 0, 195, 169}, 0600); err != nil {
		t.Fatal(err)
	}
	r := openTestRuntime(t, fmt.Sprintf(`wraith.fs.read_async(%q, {offset=1,max_bytes=2}, function(ctx)
 assert(ctx.data.result.data == string.char(0,195))
 assert(ctx.data.result.truncated and ctx.data.result.next_offset == 3)
 wraith.notify("exact")
end)`, path))
	m := r.Manifest()
	if err := r.Activate(m.Resources); err != nil {
		t.Fatal(err)
	}
	pending, err := r.SubmitResource(receiveResource(t, r), nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	got := receiveResult(t, pending)
	if got.Err != nil || len(got.Effects) != 1 || got.Effects[0].Argument != "exact" {
		t.Fatalf("binary read callback: %+v", got)
	}
}
