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
