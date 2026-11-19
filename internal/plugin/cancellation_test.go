package plugin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/erdajt/wraith/internal/config"
)

func TestCancellationReleasesCallerBeforeNativeReadReturns(t *testing.T) {
	for _, origin := range []string{"close", "parent"} {
		t.Run(origin, func(t *testing.T) {
			dir := t.TempDir()
			fifo := filepath.Join(dir, "pipe")
			if err := syscall.Mkfifo(fifo, 0600); err != nil {
				t.Fatal(err)
			}
			source := fmt.Sprintf(`wraith.commands.create("read", function()
local file = assert(io.open(%q, "r")); file:read("*l"); file:close()
end)`, fifo)
			if err := os.WriteFile(filepath.Join(dir, "init.lua"), []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			t.Cleanup(cancel)
			r, err := Open(ctx, dir, config.Default(), nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(r.Close)
			workerStopped := make(chan struct{})
			id, err := r.services.reserve("config", "fixture", func(ctx context.Context, _ *resourceSink) resourceResult {
				defer close(workerStopped)
				<-ctx.Done()
				return resourceResult{Err: ctx.Err()}
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := r.Activate([]string{id}); err != nil {
				t.Fatal(err)
			}
			m := r.Manifest()
			result, err := r.Submit(m.Commands[0].Callback, "", nil, m.Config)
			if err != nil {
				t.Fatal(err)
			}
			deadline := time.After(time.Second)
			tick := time.NewTicker(time.Millisecond)
			defer tick.Stop()
			var writer *os.File
			for {
				writer, err = os.OpenFile(fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0600)
				if err == nil {
					break
				}
				select {
				case <-tick.C:
				case <-deadline:
					t.Fatal("native callback did not start")
				}
			}
			if origin == "close" {
				r.Close()
			} else {
				cancel()
			}
			select {
			case got := <-result:
				if !errors.Is(got.Err, context.Canceled) {
					t.Fatalf("cancelled result: %+v", got)
				}
			case <-time.After(time.Second):
				t.Fatal("caller waited for native read")
			}

			select {
			case <-workerStopped:
			case <-time.After(time.Second):
				t.Fatal("managed worker waited for the blocked Lua VM")
			}

			_, _ = writer.WriteString("done\n")
			_ = writer.Close()
			select {
			case <-r.Done():
			case <-time.After(time.Second):
				t.Fatal("worker did not close after native read completed")
			}
			select {
			case duplicate := <-result:
				t.Fatalf("caller resolved twice: %+v", duplicate)
			default:
			}
		})
	}
}

func TestFailedSetupRunsRegisteredCleanup(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "cleanup")
	module := fmt.Sprintf(`return {setup = function() end, teardown = function()
local file = assert(io.open(%q, "w")); file:write("closed"); file:close()
end}`, marker)
	for name, source := range map[string]string{
		"resource.lua": module,
		"init.lua":     `wraith.plugins.load("resource"); error("failed configuration")`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if r, err := Open(t.Context(), dir, config.Default(), nil); err == nil {
		r.Close()
		t.Fatal("failed configuration accepted")
	}
	content, err := os.ReadFile(marker)
	if err != nil || string(content) != "closed" {
		t.Fatalf("cleanup result: %q, %v", content, err)
	}
}
