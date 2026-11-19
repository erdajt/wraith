package plugin

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"testing"
	"time"
)

// TestManagedProcessFixture runs only in child test processes started by the process integration tests.
func TestManagedProcessFixture(t *testing.T) {
	if os.Getenv("WRAITH_PROCESS_FIXTURE") != "1" {
		return
	}
	mode := os.Args[len(os.Args)-1]
	switch mode {
	case "echo":
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			os.Exit(8)
		}
		for _, part := range [][]byte{data[:min(2, len(data))], data[min(2, len(data)):]} {
			_, _ = os.Stdout.Write(part)
		}
		_, _ = os.Stderr.Write([]byte("diagnostic\r\n"))
		os.Exit(7)
	case "wait":
		_, _ = fmt.Fprintln(os.Stdout, "ready")
		for {
			time.Sleep(time.Hour)
		}
	default:
		os.Exit(9)
	}
}

// processFixture starts a real subprocess through the same manager used by Lua plugins.
func processFixture(t *testing.T, mode string, timeout time.Duration) (*resourceManager, *managedProcess, string) {
	t.Helper()
	manager := newResourceManager(t.Context())
	t.Cleanup(manager.close)
	process := newManagedProcess(processOptions{
		argv:        []string{os.Args[0], "-test.run=^TestManagedProcessFixture$", "--", mode},
		environment: append(os.Environ(), "WRAITH_PROCESS_FIXTURE=1"),
		limit:       128 << 10, protocol: true, stdin: true, timeout: timeout,
	})
	id, err := manager.reserve("config", "process", process.run)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.activate(id); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-manager.events:
		if event.Kind != "started" {
			t.Fatalf("process did not start: %+v", event)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("process startup timed out")
	}
	return manager, process, id
}

func TestManagedProcessStreamsStdinAndNonzeroExit(t *testing.T) {
	manager, process, _ := processFixture(t, "echo", 0)
	want := []byte{0xe7, 0x95, 0x8c, 0, 255, '\r', '\n'}
	if err := process.enqueue(want, false); err != nil {
		t.Fatal(err)
	}
	if err := process.enqueue(nil, true); err != nil {
		t.Fatal(err)
	}
	if err := process.enqueue([]byte("late"), false); err == nil {
		t.Fatal("write after stdin close was accepted")
	}
	streams := map[string][]byte{}
	for {
		select {
		case event := <-manager.events:
			if event.Final {
				result := event.Data["result"].(map[string]any)
				if event.Data["state"] != "failed" || result["exit_code"] != float64(7) {
					t.Fatalf("nonzero exit lost: %+v", event)
				}
				if !bytes.Equal(streams["stdout"], want) || string(streams["stderr"]) != "diagnostic\r\n" {
					t.Fatalf("stream bytes changed: %#v", streams)
				}
				window, err := process.output("stdout", 0, 1024)
				if err != nil || !bytes.Equal(window["data"].([]byte), want) {
					t.Fatalf("retained output differs: %v, %v", window, err)
				}
				return
			}
			if data, ok := event.Data["data"].([]byte); ok {
				streams[event.Kind] = append(streams[event.Kind], data...)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("process completion timed out")
		}
	}
}

func TestManagedProcessCancellationClosesBlockedStdin(t *testing.T) {
	manager, process, id := processFixture(t, "wait", 0)
	var err error
	for range 100 {
		err = process.enqueue(bytes.Repeat([]byte{'x'}, 64<<10), false)
		if err != nil {
			break
		}
	}
	if err == nil {
		t.Fatal("stdin queue did not enforce its bound")
	}
	if err := manager.Cancel(id, "config"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-process.inputDone:
	case <-time.After(5 * time.Second):
		t.Fatal("cancellation did not unblock stdin")
	}
}

func TestManagedProcessTimeoutAndLaunchFailure(t *testing.T) {
	for _, timeout := range []time.Duration{100 * time.Millisecond, time.Second} {
		t.Run(strconv.FormatInt(int64(timeout), 10), func(t *testing.T) {
			manager := newResourceManager(t.Context())
			t.Cleanup(manager.close)
			options := processOptions{argv: []string{"/nonexistent/wraith-process-fixture"}, limit: 1024, timeout: timeout}
			if timeout == 100*time.Millisecond {
				options.argv = []string{os.Args[0], "-test.run=^TestManagedProcessFixture$", "--", "wait"}
				options.environment = append(os.Environ(), "WRAITH_PROCESS_FIXTURE=1")
			}
			process := newManagedProcess(options)
			id, err := manager.reserve("config", "process", process.run)
			if err != nil {
				t.Fatal(err)
			}
			if err := manager.activate(id); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			for {
				select {
				case event := <-manager.events:
					if event.Final {
						if event.Data["state"] != "failed" || event.Data["error"] == nil {
							t.Fatalf("failure was hidden: %+v", event)
						}
						return
					}
				case <-ctx.Done():
					t.Fatal("failed process did not complete")
				}
			}
		})
	}
}
