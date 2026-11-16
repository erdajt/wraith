package runner

import (
	"strings"
	"testing"
	"time"

	"github.com/erdajt/wraith/internal/variables"
)

func TestOutputSearchRedactsBeforeMatchingAndSupportsPagination(t *testing.T) {
	t.Parallel()
	record := Record{ShareOutput: true, Finished: time.Now(), Output: "first\nprivate.example ready\nneighbor\nprivate.example failed\nlast\n"}
	snapshot := variables.NewSnapshot([]variables.Entry{{Name: "TARGET", Value: "private.example"}}, nil)
	page, err := ReadOutput(record, Query{Pattern: "wraith::TARGET", After: 1, Limit: 2}, snapshot)
	if err != nil || len(page.Lines) != 2 || page.Lines[0].Number != 2 || page.Next != 4 {
		t.Fatalf("unexpected page: %+v %v", page, err)
	}
	next, err := ReadOutput(record, Query{Pattern: "wraith::TARGET", After: 1, From: page.Next, Limit: 2}, snapshot)
	if err != nil || len(next.Lines) != 2 || next.Lines[0].Number != 4 || next.Next != 0 {
		t.Fatalf("unexpected next page: %+v %v", next, err)
	}
	for _, line := range append(page.Lines, next.Lines...) {
		if strings.Contains(line.Text, "private.example") {
			t.Fatal("literal escaped redaction")
		}
	}
	hidden, err := ReadOutput(record, Query{Pattern: "private.example"}, snapshot)
	if err != nil || len(hidden.Lines) != 0 {
		t.Fatal("search matched before redaction")
	}
}

func TestOutputQueryNeverExposesPartialSecretsOrHiddenJobs(t *testing.T) {
	t.Parallel()
	snapshot := variables.NewSnapshot([]variables.Entry{{Name: "TARGET", Value: "private.example"}}, nil)
	record := Record{ShareOutput: true, Output: "complete\nprivate.exa"}
	page, err := ReadOutput(record, Query{}, snapshot)
	if err != nil || len(page.Lines) != 1 || page.Lines[0].Text != "complete" {
		t.Fatalf("partial live line exposed: %+v %v", page, err)
	}
	record.OutputHidden = true
	if _, err := ReadOutput(record, Query{}, snapshot); err == nil {
		t.Fatal("hidden output was readable")
	}
	record.OutputHidden = false
	record.ShareOutput = false
	if _, err := ReadOutput(record, Query{}, snapshot); err == nil {
		t.Fatal("legacy private output was readable")
	}
}

func TestOutputQueryRetainsTailLineNumbersAndDropsTruncatedEdges(t *testing.T) {
	t.Parallel()
	record := Record{ShareOutput: true, Truncated: true, Finished: time.Now(), Output: "head\nprivate.exa", Tail: "ample\ntail\nfinal", TailOffset: 100, TailLine: 20}
	page, err := ReadOutput(record, Query{}, variables.Snapshot{})
	if err != nil || len(page.Lines) != 3 || page.Lines[1].Number != 21 || page.Lines[2].Text != "final" {
		t.Fatalf("incorrect retained ranges: %+v %v", page, err)
	}
}

func TestOutputQueryBoundsRegexAndOversizedLines(t *testing.T) {
	t.Parallel()
	record := Record{ShareOutput: true, Finished: time.Now(), Output: strings.Repeat("x", 1<<20+1) + "\nmatch 42\n"}
	page, err := ReadOutput(record, Query{Regex: true, Pattern: `match \d+`}, variables.Snapshot{})
	if err != nil || len(page.Lines) != 1 || page.Lines[0].Number != 2 {
		t.Fatalf("regex lost complete later line: %+v %v", page, err)
	}
	for _, query := range []Query{{Regex: true, Pattern: "["}, {Before: 11}, {Limit: 201}, {From: -1}} {
		if _, err := ReadOutput(record, query, variables.Snapshot{}); err == nil {
			t.Fatal("invalid query accepted")
		}
	}
}
