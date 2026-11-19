package runner

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestOutputStreamMergesOverlappingCapturesAndContext(t *testing.T) {
	record := Record{Finished: time.Now(), Output: "old 1\nold 2\nold 3\nold 4\nold 5\n", Tail: "partial\nnew 3\nmatch 4\nnew 5\nmatch 6\nnew 7\n", TailOffset: 3, TailLine: 2}
	page, err := ReadLocalOutput(record, Query{Pattern: "match", Before: 1, After: 1, Limit: 3})
	want := []OutputLine{{Number: 3, Text: "new 3"}, {Number: 4, Text: "match 4"}, {Number: 5, Text: "new 5"}}
	if err != nil || !reflect.DeepEqual(page.Lines, want) || page.Next != 6 || page.Stats.RetainedLines != 7 || page.Stats.MatchedLines != 2 {
		t.Fatalf("overlap/context page: %+v %v", page, err)
	}
	next, err := ReadLocalOutput(record, Query{Pattern: "match", Before: 1, After: 1, From: page.Next})
	if err != nil || len(next.Lines) != 2 || next.Lines[0].Number != 6 || next.Lines[1].Number != 7 || next.Next != 0 {
		t.Fatalf("continuation: %+v %v", next, err)
	}
}

func TestOutputStreamKeepsNumericContextAcrossCaptureGaps(t *testing.T) {
	record := Record{Finished: time.Now(), Truncated: true, Output: "head\nmatch\n", Tail: "partial\ntail\nmatch\nlast\n", TailOffset: 100, TailLine: 30}
	page, err := ReadLocalOutput(record, Query{Pattern: "match", Before: 2, After: 2})
	want := []int64{1, 2, 31, 32, 33}
	var got []int64
	for _, line := range page.Lines {
		got = append(got, line.Number)
	}
	if err != nil || !reflect.DeepEqual(got, want) || page.Stats.RetainedLines != 5 {
		t.Fatalf("gap context: %+v %v", page, err)
	}
}

func TestOutputStreamBoundsResultWhileCountingAllLines(t *testing.T) {
	record := Record{Finished: time.Now(), Output: strings.Repeat("\n", 100000)}
	page, err := ReadLocalOutput(record, Query{Limit: 2})
	if err != nil || len(page.Lines) != 2 || page.Next != 3 || page.Stats.RetainedLines != 100000 || page.Stats.MatchedLines != 100000 {
		t.Fatalf("empty-line capture: %+v %v", page, err)
	}
	record.Output = strings.Repeat("x", 9000) + "\n" + strings.Repeat("y", 9000) + "\n"
	page, err = ReadLocalOutput(record, Query{})
	if err != nil || len(page.Lines) != 1 || page.Next != 2 {
		t.Fatalf("byte-limited page: %+v %v", page, err)
	}
}

func TestOutputStreamCancellationAndRegexBudgets(t *testing.T) {
	record := Record{Finished: time.Now(), Output: strings.Repeat("x\n", 100000)}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := ReadLocalOutputContext(ctx, record, Query{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	record.Output = strings.Repeat(strings.Repeat("y", 512)+"\n", 512)
	for _, pattern := range []string{"(" + strings.Repeat("a", 900) + "){1000}", strings.Repeat("x", 512)} {
		if _, err := ReadLocalOutput(record, Query{Pattern: pattern, Regex: true}); err == nil || !strings.Contains(err.Error(), "limit") {
			t.Fatalf("regex budget: %v", err)
		}
	}
}

func BenchmarkOutputStreamEmptyLines(b *testing.B) {
	for _, count := range []int{1000, 100000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			record := Record{Finished: time.Now(), Output: strings.Repeat("\n", count)}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				page, err := ReadLocalOutput(record, Query{Before: 10, After: 10, Limit: 2})
				if err != nil || page.Stats.RetainedLines != count {
					b.Fatalf("search: %+v %v", page, err)
				}
			}
		})
	}
}
