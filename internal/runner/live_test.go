package runner

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/erdajt/wraith/internal/session"
)

func TestRecoverySkipsOwnedJobsAndRetainsCheckpointedOutput(t *testing.T) {
	t.Parallel()
	history := History{Root: t.TempDir()}
	record := Record{ID: session.ID(), Session: session.ID(), Command: "never rerun", Status: "running", Started: time.Now()}
	live, err := history.openLive(record)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(live.close)
	output := &capture{}
	_, _ = output.Write([]byte("saved output\n"))

	if _, err = live.prefix.WriteString(output.text.String()); err != nil {
		t.Fatal(err)
	}

	if err = live.checkpoint(record, output); err != nil {
		t.Fatal(err)
	}

	if recovered, err := history.Recover(); err != nil || len(recovered) != 0 {
		t.Fatalf("recovered another live owner: %v", err)
	}

	live.close()
	live.lock, live.prefix = nil, nil
	recovered, err := history.Recover()
	if err != nil || len(recovered) != 1 || recovered[0].Status != "interrupted" || recovered[0].Output != "saved output\n" {
		t.Fatalf("bad recovery: %+v %v", recovered, err)
	}

	saved, err := history.Load(record.ID)
	if err != nil || saved.Tail != saved.Output || saved.ExitCode != -1 {
		t.Fatalf("incomplete durable result: %+v %v", saved, err)
	}

	if _, err = os.Stat(live.directory); !os.IsNotExist(err) {
		t.Fatal("recovered journal retained")
	}
}

func TestRecoveryDoesNotOverwriteFinalResults(t *testing.T) {
	t.Parallel()
	h := History{Root: t.TempDir()}
	record := Record{ID: session.ID(), Status: "running"}
	live, err := h.openLive(record)
	if err != nil {
		t.Fatal(err)
	}

	live.close()
	record.Status, record.Finished, record.Output = "completed", time.Now(), "final"

	if err = h.Save(record); err != nil {
		t.Fatal(err)
	}

	if recovered, err := h.Recover(); err != nil || len(recovered) != 0 {
		t.Fatalf("recovered completed command: %v", err)
	}

	got, err := h.Load(record.ID)
	if err != nil || got.Output != "final" || got.Status != "completed" {
		t.Fatal("final result overwritten")
	}
}

func TestRecoveryRejectsCorruptMetadataWithoutRemovingEvidence(t *testing.T) {
	t.Parallel()
	h := History{Root: t.TempDir()}
	live, err := h.openLive(Record{ID: session.ID(), Status: "running"})
	if err != nil {
		t.Fatal(err)
	}

	live.close()
	path := filepath.Join(live.directory, "record.json")
	if err = os.WriteFile(path, []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err = h.Recover(); err == nil {
		t.Fatal("corrupt recovery silently accepted")
	}

	if _, err = os.Stat(path); err != nil {
		t.Fatal("corrupt recovery evidence removed")
	}
}

func TestRunningCommandCheckpointsOutputAndRemovesJournalAtCompletion(t *testing.T) {
	t.Parallel()
	h := History{Root: t.TempDir()}
	id := session.ID()
	record, err := h.Run(t.Context(), Record{ID: id, Shell: "/bin/sh", Command: "printf ready"}, func(string) {
		recovered, err := h.Recover()
		if err != nil || len(recovered) != 0 {
			t.Errorf("live command recovered: %v", err)
		}
	})
	if err != nil || record.Output != "ready" {
		t.Fatalf("run failed: %+v %v", record, err)
	}

	if _, err := os.Stat(filepath.Join(h.Root, "live", id)); !os.IsNotExist(err) {
		t.Fatal("final command retained live journal")
	}
}

func TestRecoveryPreservesTerminalCheckpointBeforeFinalSave(t *testing.T) {
	t.Parallel()
	h := History{Root: t.TempDir()}
	record := Record{ID: session.ID(), Status: "failed", ExitCode: 7, Finished: time.Now().UTC(), Error: "exit status 7"}
	live, err := h.openLive(record)
	if err != nil {
		t.Fatal(err)
	}

	live.close()
	recovered, err := h.Recover()
	if err != nil || len(recovered) != 1 {
		t.Fatalf("recovery: %v %v", recovered, err)
	}

	if got := recovered[0]; got.Status != record.Status || got.ExitCode != 7 || !got.Finished.Equal(record.Finished) || got.Error != record.Error {
		t.Fatalf("terminal checkpoint changed: %+v", got)
	}
}

func TestSetupFailureCannotLeaveRunningIntent(t *testing.T) {
	t.Parallel()
	h := History{Root: t.TempDir()}
	if err := os.WriteFile(filepath.Join(h.Root, "live"), nil, 0600); err != nil {
		t.Fatal(err)
	}

	record, err := h.Run(t.Context(), Record{ID: session.ID(), Shell: "/bin/sh", Command: "true"}, nil)
	if err == nil || record.Status != "failed" || record.Finished.IsZero() {
		t.Fatalf("setup: %+v %v", record, err)
	}

	if _, err := h.Load(record.ID); !os.IsNotExist(err) {
		t.Fatalf("orphan intent: %v", err)
	}
}

func TestPreparationOwnsJournalBeforeSavingIntent(t *testing.T) {
	t.Parallel()
	h := History{Root: t.TempDir()}
	record, live, err := h.prepare(Record{ID: session.ID(), Shell: "/bin/sh", Command: "true"})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := h.Load(record.ID); err != nil {
		live.close()
		t.Fatal(err)
	}

	if recovered, err := h.Recover(); err != nil || len(recovered) != 0 {
		live.close()
		t.Fatalf("owner missing: %v", err)
	}

	live.close()
	recovered, err := h.Recover()
	if err != nil || len(recovered) != 1 || recovered[0].Status != "interrupted" {
		t.Fatalf("orphan intent: %v %v", recovered, err)
	}
}

func TestRecoveryIgnoresUnpublishedInitialization(t *testing.T) {
	t.Parallel()
	h := History{Root: t.TempDir()}
	parent := filepath.Join(h.Root, "live")
	if err := os.MkdirAll(parent, 0700); err != nil {
		t.Fatal(err)
	}
	staging, err := os.MkdirTemp(parent, ".starting-")
	if err != nil {
		t.Fatal(err)
	}
	owner := filepath.Join(staging, "owner.lock")
	if err = os.WriteFile(owner, nil, 0600); err != nil {
		t.Fatal(err)
	}

	if records, err := h.Recover(); err != nil || len(records) != 0 {
		t.Fatalf("initializing journal recovered: %v", err)
	}

	if _, err = os.Stat(owner); err != nil {
		t.Fatal("initializing journal deleted")
	}

	record := Record{ID: session.ID(), Status: "starting"}
	live, err := h.openLive(record)
	if err != nil {
		t.Fatal(err)
	}
	defer live.close()

	if _, err := h.openLive(record); err == nil {
		t.Fatal("existing journal replaced")
	}

	if records, err := h.Recover(); err != nil || len(records) != 0 {
		t.Fatalf("published owner lost: %v", err)
	}

	if _, err = os.Stat(filepath.Join(live.directory, "record.json")); err != nil {
		t.Fatal("published checkpoint missing")
	}
}
