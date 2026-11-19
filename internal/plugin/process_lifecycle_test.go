package plugin

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// awaitProcessDone waits for actual reaping rather than the earlier cancellation notification.
func awaitProcessDone(t *testing.T, manager *resourceManager, id string) {
	t.Helper()
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		status, err := manager.Status(id, "config")
		if err != nil {
			t.Fatal(err)
		}
		if status["done"] == true {
			return
		}
		select {
		case <-tick.C:
		case <-timeout.C:
			t.Fatal("process was not reaped")
		}
	}
}

func TestManagedProcessStopsOrdinaryDescendants(t *testing.T) {
	for _, ending := range []string{"cancel", "timeout", "parent exit"} {
		t.Run(ending, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "child.pid")
			mode, timeout := "child", time.Duration(0)
			if ending == "timeout" {
				timeout = 500 * time.Millisecond
			} else if ending == "parent exit" {
				mode = "orphan"
			}
			manager, _, id := processFixture(t, mode, timeout, "WRAITH_CHILD_PID="+path)
			deadline := time.NewTimer(5 * time.Second)
			defer deadline.Stop()
			cancelled := false
			for {
				select {
				case event := <-manager.events:
					if !cancelled && ending == "cancel" {
						if _, err := os.Stat(path); err == nil {
							if err := manager.Cancel(id, "config"); err != nil {
								t.Fatal(err)
							}
							cancelled = true
						}
					}
					if event.Final {
						awaitProcessDone(t, manager, id)
						data, err := os.ReadFile(path)
						if err != nil {
							t.Fatal(err)
						}
						pid, err := strconv.Atoi(string(data))
						if err != nil {
							t.Fatal(err)
						}
						stat, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
						if err != nil && !os.IsNotExist(err) {
							t.Fatal(err)
						}
						if err == nil {
							state := string(stat)[strings.LastIndex(string(stat), ") ")+2:]
							if !strings.HasPrefix(state, "Z ") && !strings.HasPrefix(state, "X ") {
								t.Fatalf("descendant remains alive: %s", stat)
							}
						}
						return
					}
				case <-deadline.C:
					t.Fatal("descendant fixture did not complete")
				}
			}
		})
	}
}

func TestManagedProcessCancellationUnblocksUnreadProtocolOutput(t *testing.T) {
	manager, _, id := processFixture(t, "flood", 0)
	full := time.NewTimer(5 * time.Second)
	defer full.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		manager.mu.Lock()
		queued := manager.progress
		manager.mu.Unlock()
		if queued == 256 {
			break
		}
		select {
		case <-tick.C:
		case <-full.C:
			t.Fatal("protocol fixture did not reach backpressure")
		}
	}
	if err := manager.Cancel(id, "config"); err != nil {
		t.Fatal(err)
	}
	awaitProcessDone(t, manager, id)
}
