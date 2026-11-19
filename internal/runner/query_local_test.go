package runner

import (
	"testing"
	"time"

	"github.com/erdajt/wraith/internal/variables"
)

// TestTrustedOutputPreservesModelPrivacy keeps local inspection separate from agent permissions.
func TestTrustedOutputPreservesModelPrivacy(t *testing.T) {
	record := Record{Output: "private.example\nother\n", OutputHidden: true, ShareOutput: false, Finished: time.Now()}
	page, err := ReadLocalOutput(record, Query{Pattern: "private"})
	if err != nil || len(page.Lines) != 1 || page.Lines[0].Text != "private.example" {
		t.Fatalf("trusted output = %#v, %v", page, err)
	}
	if _, err := ReadOutput(record, Query{}, variables.Snapshot{}); err == nil {
		t.Fatal("trusted read enabled agent output")
	}
	if record.ShareOutput || !record.OutputHidden {
		t.Fatal("trusted read changed sharing flags")
	}
}
