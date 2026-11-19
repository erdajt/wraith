package plugin

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// managerPublication writes private transaction files alongside an authoritative lock.
func managerPublication(t *testing.T, directory, id, baseline string) (string, string) {
	t.Helper()
	before, candidate := filepath.Join(directory, id+".baseline"), filepath.Join(directory, id+".candidate")
	for path, data := range map[string]string{before: baseline, candidate: `{"version":1,"generation":"` + id + `","packages":{}}`} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return before, candidate
}

// managerPublishRuntime invokes the shipped transaction through actual managed flock and shell processes.
func managerPublishRuntime(t *testing.T, directory, id, before, candidate, expected string, absent bool) *Runtime {
	t.Helper()
	return openManagerRuntime(t, fmt.Sprintf(`require("wraith.manager.store_publish").commit({
 lockfile=%q,guard=%q,baseline=%q,candidate=%q,generation=%q,absent=%t}, {},function(result)
 assert(result.state==%q,result.error)
 wraith.notify("complete")
end)`, filepath.Join(directory, "lock.json"), filepath.Join(directory, ".guard"), before, candidate, id, absent, expected))
}

// TestManagerPublicationRejectsStaleBaselines lets exactly one competing transaction replace the graph.
func TestManagerPublicationRejectsStaleBaselines(t *testing.T) {
	directory := t.TempDir()
	baseline := `{"version":1,"generation":"initial","packages":{}}`
	for path, data := range map[string]string{"lock.json": baseline, ".guard": ""} {
		if err := os.WriteFile(filepath.Join(directory, path), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	firstBefore, firstCandidate := managerPublication(t, directory, "first", baseline)
	secondBefore, secondCandidate := managerPublication(t, directory, "second", baseline)
	first := managerPublishRuntime(t, directory, "first", firstBefore, firstCandidate, "installed", false)
	second := managerPublishRuntime(t, directory, "second", secondBefore, secondCandidate, "conflict", false)
	runManagerResources(t, first)
	runManagerResources(t, second)
	data, err := os.ReadFile(filepath.Join(directory, "lock.json"))
	if err != nil || !strings.Contains(string(data), `"generation":"first"`) {
		t.Fatalf("published graph: %s, %v", data, err)
	}
	if _, err := os.Stat(secondCandidate); err != nil {
		t.Fatalf("conflicting candidate lost: %v", err)
	}
}

// TestManagerPublicationHandlesAbsentAndInvalidDestinations refuses a directory or symlink lock.
func TestManagerPublicationHandlesAbsentAndInvalidDestinations(t *testing.T) {
	for _, kind := range []string{"absent", "directory", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			directory := t.TempDir()
			if err := os.WriteFile(filepath.Join(directory, ".guard"), nil, 0600); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(directory, "lock.json")
			switch kind {
			case "directory":
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Symlink("missing", path); err != nil {
					t.Fatal(err)
				}
			}
			before, candidate := managerPublication(t, directory, "created", "")
			expected := "installed"
			if kind != "absent" {
				expected = "failed"
			}
			runManagerResources(t, managerPublishRuntime(t, directory, "created", before, candidate, expected, true))
		})
	}
}

// TestManagerPublicationSerializesConcurrentChildren commits one complete graph across two live runtimes.
func TestManagerPublicationSerializesConcurrentChildren(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, ".guard"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	t.Run("publishers", func(t *testing.T) {
		for _, id := range []string{"one", "two"} {
			t.Run(id, func(t *testing.T) {
				t.Parallel()
				before, candidate := managerPublication(t, directory, id, "")
				r := openManagerRuntime(t, fmt.Sprintf(`require("wraith.manager.store_publish").commit({
 lockfile=%q,guard=%q,baseline=%q,candidate=%q,generation=%q,absent=true},{},function(result)
 assert(result.state=="installed" or result.state=="conflict",result.error)
 wraith.fs.write(%q,result.state)
 wraith.notify("complete")
end)`, filepath.Join(directory, "lock.json"), filepath.Join(directory, ".guard"), before, candidate, id, filepath.Join(directory, id+".result")))
				runManagerResources(t, r)
			})
		}
	})
	states := map[string]int{}
	for _, id := range []string{"one", "two"} {
		data, err := os.ReadFile(filepath.Join(directory, id+".result"))
		if err != nil {
			t.Fatal(err)
		}
		states[string(data)]++
	}
	if states["installed"] != 1 || states["conflict"] != 1 {
		t.Fatalf("concurrent outcomes: %v", states)
	}
}

// TestManagerPublicationTimesOutAndRecoversAfterHolderExit uses a real independent lock holder.
func TestManagerPublicationTimesOutAndRecoversAfterHolderExit(t *testing.T) {
	directory := t.TempDir()
	guard := filepath.Join(directory, ".guard")
	holder := exec.Command("flock", "--no-fork", "--exclusive", guard, "/bin/sh", "-c", "printf ready; read ignored")
	input, err := holder.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := holder.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := holder.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = input.Close(); _ = holder.Wait() }()
	ready := make([]byte, 5)
	if _, err := io.ReadFull(output, ready); err != nil || string(ready) != "ready" {
		t.Fatalf("lock holder: %q, %v", ready, err)
	}
	before, candidate := managerPublication(t, directory, "waiting", "")
	r := openManagerRuntime(t, fmt.Sprintf(`require("wraith.manager.store_publish").commit({
 lockfile=%q,guard=%q,baseline=%q,candidate=%q,generation="waiting",absent=true},{lock_timeout_ms=20},function(result)
 assert(result.state=="failed" and result.error:find("timed out",1,true),result.error)
 wraith.notify("complete")
end)`, filepath.Join(directory, "lock.json"), guard, before, candidate))
	runManagerResources(t, r)
	_ = input.Close()
	_ = holder.Wait()
	runManagerResources(t, managerPublishRuntime(t, directory, "waiting", before, candidate, "installed", true))
}

// TestManagerPublicationReconcilesCancellation distinguishes untouched and already committed graphs.
func TestManagerPublicationReconcilesCancellation(t *testing.T) {
	for _, committed := range []bool{false, true} {
		t.Run(fmt.Sprint(committed), func(t *testing.T) {
			directory := t.TempDir()
			guard := filepath.Join(directory, ".guard")
			if err := os.WriteFile(guard, nil, 0600); err != nil {
				t.Fatal(err)
			}
			before, candidate := managerPublication(t, directory, "cancelled", "")
			wait := "operation.cancel()"
			expected := "cancelled"
			if committed {
				bin := t.TempDir()
				mv, err := exec.LookPath("mv")
				if err != nil {
					t.Fatal(err)
				}
				program := "#!/bin/sh\n'" + strings.ReplaceAll(mv, "'", "'\\''") + "' \"$@\" || exit $?\nprintf committed\nsleep 10\n"
				if err := os.WriteFile(filepath.Join(bin, "mv"), []byte(program), 0700); err != nil {
					t.Fatal(err)
				}
				t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
				expected = "installed"
				wait = `local timer
 timer=wraith.timer.start(10,function()
  if operation.output("stdout").data=="committed" then
   wraith.resource.cancel(timer); wraith.resource.release(timer); operation.cancel()
  end
 end,{repeat_ms=10})`
			}
			r := openManagerRuntime(t, fmt.Sprintf(`local operation=require("wraith.manager.store_publish").commit({
 lockfile=%q,guard=%q,baseline=%q,candidate=%q,generation="cancelled",absent=true},{},function(result)
 assert(result.state==%q,result.error)
 wraith.notify("complete")
end)
%s`, filepath.Join(directory, "lock.json"), guard, before, candidate, expected, wait))
			runManagerResources(t, r)
			_, err := os.Stat(filepath.Join(directory, "lock.json"))
			if committed && err != nil || !committed && !os.IsNotExist(err) {
				t.Fatalf("wrong cancellation outcome: %v", err)
			}
		})
	}
}
