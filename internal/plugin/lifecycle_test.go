package plugin

import (
	"context"
	"github.com/erdajt/wraith/internal/config"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestConfigSetupCanClearArray(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "init.lua"), []byte(`wraith.config.setup({clipboard = {}})`), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := Open(t.Context(), dir, config.Default(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if len(r.Manifest().Config.Clipboard) != 0 {
		t.Fatal("clipboard array not cleared")
	}
}

func TestCoroutineCreatedDuringSetupRunsInCallback(t *testing.T) {
	dir := t.TempDir()
	source := `local co=coroutine.create(function() return 42 end)
wraith.commands.create("check",function() local ok,v=coroutine.resume(co); assert(ok,v); assert(v==42) end)`
	if err := os.WriteFile(filepath.Join(dir, "init.lua"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := Open(t.Context(), dir, config.Default(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	m := r.Manifest()
	ch, err := r.Submit(m.Commands[0].Callback, "", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	if result := <-ch; result.Err != nil {
		t.Fatal(result.Err)
	}
}

func TestFIFORejectedBeforeBlockingOpen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "init.lua")
	if err := syscall.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		r, err := Open(context.Background(), dir, config.Default(), nil)
		if r != nil {
			r.Close()
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("fifo accepted")
		}
	case <-time.After(2300 * time.Millisecond):
		fd, err := syscall.Open(path, syscall.O_WRONLY|syscall.O_NONBLOCK, 0600)
		if err == nil {
			_ = syscall.Close(fd)
		}
		select {
		case <-done:
		case <-time.After(time.Second):
		}
		t.Fatal("nonregular lua source blocked past startup deadline")
	}
}

func TestWrappedCoroutineSurvivesSeparateCallbacks(t *testing.T) {
	dir := t.TempDir()
	source := `local next = coroutine.wrap(function() coroutine.yield(1); return 2 end)
 local expected = 0
 wraith.commands.create("step", function() expected = expected + 1; assert(next() == expected) end)`
	if err := os.WriteFile(filepath.Join(dir, "init.lua"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := Open(t.Context(), dir, config.Default(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	m := r.Manifest()
	for range 2 {
		result, err := r.Submit(m.Commands[0].Callback, "", nil, m.Config)
		if err != nil {
			t.Fatal(err)
		}
		if got := <-result; got.Err != nil {
			t.Fatal(got.Err)
		}
	}
}

func TestCloseReleasesQueuedCallbacks(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "init.lua"), []byte(`wraith.commands.create("wait",function() while true do end end)`), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := Open(t.Context(), dir, config.Default(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	m := r.Manifest()
	var results []<-chan Result
	for range 16 {
		result, err := r.Submit(m.Commands[0].Callback, "", nil, m.Config)
		if err != nil {
			t.Fatal(err)
		}
		results = append(results, result)
	}
	r.Close()
	for _, result := range results {
		select {
		case got := <-result:
			if got.Err == nil {
				t.Fatal("cancelled callback succeeded")
			}
		case <-time.After(time.Second):
			t.Fatal("queued caller remained blocked")
		}
	}
}
