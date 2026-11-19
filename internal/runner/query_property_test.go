package runner

import (
	"math/rand/v2"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestOutputStreamMatchesIndexedLiteralSearch(t *testing.T) {
	random := rand.New(rand.NewPCG(73, 19))
	capture := func() string {
		var text strings.Builder
		for range random.IntN(20) {
			text.WriteString([]string{"hit", "miss", "hit again", ""}[random.IntN(4)])
			text.WriteByte('\n')
		}
		if random.IntN(2) == 1 {
			text.WriteString("hit partial")
		}
		return text.String()
	}
	for iteration := range 1000 {
		record := Record{Output: capture(), Tail: capture(), TailOffset: 100, TailLine: int64(1 + random.IntN(25)), Truncated: random.IntN(2) == 1}
		if random.IntN(2) == 1 {
			record.Finished = time.Now()
		}
		query := Query{Pattern: "hit", Before: random.IntN(5), After: random.IntN(5), From: int64(random.IntN(25)), Limit: 1 + random.IntN(8)}
		want := indexedLiteralPage(record, query)
		got, err := ReadLocalOutput(record, query)
		if err != nil || !reflect.DeepEqual(got.Lines, want.Lines) || got.Next != want.Next || got.Stats.RetainedLines != want.Stats.RetainedLines || got.Stats.MatchedLines != want.Stats.MatchedLines {
			t.Fatalf("case %d record=%+v query=%+v\ngot=%+v err=%v\nwant=%+v", iteration, record, query, got, err, want)
		}
	}
}

// indexedLiteralPage is a small allocation-heavy oracle for streaming range and pagination behavior.
func indexedLiteralPage(record Record, query Query) Page {
	lines := map[int64]string{}
	for index, text := range []string{record.Output, record.Tail} {
		number := int64(1)
		if index == 1 {
			number = record.TailLine
		}
		first := true
		for text != "" {
			line, rest, terminated := strings.Cut(text, "\n")
			text = rest
			if !terminated && (record.Finished.IsZero() || index == 0 && record.Truncated) {
				break
			}
			if index == 0 || !first {
				lines[number] = line
			}
			number++
			first = false
		}
	}
	page := Page{Lines: []OutputLine{}}
	page.Stats.RetainedLines = len(lines)
	selected := map[int64]bool{}
	for number, text := range lines {
		if strings.Contains(text, query.Pattern) {
			page.Stats.MatchedLines++
			for n := number - int64(query.Before); n <= number+int64(query.After); n++ {
				selected[n] = true
			}
		}
	}
	var numbers []int64
	for number := range lines {
		numbers = append(numbers, number)
	}
	sort.Slice(numbers, func(i, j int) bool { return numbers[i] < numbers[j] })
	for _, number := range numbers {
		if number < query.From || !selected[number] {
			continue
		}
		if len(page.Lines) == query.Limit {
			page.Next = number
			break
		}
		page.Lines = append(page.Lines, OutputLine{Number: number, Text: lines[number]})
	}
	return page
}
