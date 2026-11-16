package ui

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/erdajt/wraith/internal/runner"
)

func TestOutputStaysPrivateUntilExplicitSharing(t *testing.T) {
	a := fixture(t)
	a.Config.Execution.Mode = "blocks"
	a.approve(proposal{Command: "printf private-evidence", Continue: true})
	a.runApproved()
	drain(t, a)
	if len(a.Session.Messages) != 0 {
		t.Fatal("default execution shared output")
	}
	if !strings.Contains(a.View(), "share to agent") {
		t.Fatal("share binding missing")
	}
	a.loading = true
	a.menuKey(key(a.Config.Keys["output_share"]))
	if len(a.Session.Messages) != 1 || !strings.Contains(a.Session.Messages[0].Content, "private-evidence") {
		t.Fatal("explicit share lost output")
	}
	a.shareRun(a.activeRun)
	if len(a.Session.Messages) != 1 {
		t.Fatal("output was shared twice")
	}
	saved, err := a.Store.Load(a.Session.ID)
	if err != nil || len(saved.SharedRuns) != 1 {
		t.Fatal("share state not persisted")
	}
}

func TestAutomaticSharingHidesManualAction(t *testing.T) {
	a := fixture(t)
	a.Config.Execution.AutoShare = true
	a.showRun(runner.Record{ID: "record", ShareOutput: true, Finished: time.Now()}, "output")
	if strings.Contains(a.View(), "share to agent") {
		t.Fatal("automatic sharing still advertises manual action")
	}
}

func TestCompletionEvidenceReservesActualTail(t *testing.T) {
	for _, prefix := range []string{"prefix text", "前置出力"} {
		t.Run(prefix, func(t *testing.T) {
			a := fixture(t)
			a.Config.Execution.OutputContext = 512
			r := runner.Record{ShareOutput: true, Output: strings.Repeat(prefix+"\n", 40), Tail: "partial\n" + strings.Repeat("earlier tail\n", 30) + "final-error\n", TailOffset: 10000, TailLine: 1000, Truncated: true, Finished: time.Now()}
			e := a.commandEvidence(r)
			if !strings.Contains(e.Output, "final-error") || !strings.Contains(e.Output, "capture gap") || len(e.Output) > 512 || !utf8.ValidString(e.Output) {
				t.Fatalf("invalid completion excerpt: %q", e.Output)
			}
		})
	}
}
