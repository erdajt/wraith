package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/erdajt/wraith/internal/session"
)

func TestTableValueCursorAndSelectionSurviveNarrowLayouts(t *testing.T) {
	for _, value := range []string{"BRAVO", "界BRAVO界", "long value with spaces BRAVO and more"} {
		t.Run(value, func(t *testing.T) {
			a := fixture(t)
			a.Session.Messages = []session.Message{{Role: "assistant", Content: "| first | second |\n| --- | --- |\n| alpha | " + value + " |\nafter"}}
			rows := a.transcript(100)
			for i, row := range rows {
				plain := ansi.Strip(row.Text)
				if col := strings.Index(plain, "BRAVO"); col >= 0 {
					a.chatCursor, a.chatColumn = i, ansi.StringWidth(plain[:col])
				}
			}
			a.follow, a.visualMode = false, "visual"
			a.visualRow, a.visualColumn = a.chatCursor, a.chatColumn+3
			cursor := anchorAt(rows, a.chatCursor, a.chatColumn)
			selection := anchorAt(rows, a.visualRow, a.visualColumn)
			for _, width := range []int{12, 8, 30, 100} {
				a.transcript(width)
				if got := anchorAt(a.rows, a.chatCursor, a.chatColumn); got != cursor {
					t.Fatalf("width %d cursor changed: %+v -> %+v", width, cursor, got)
				}
				if got := anchorAt(a.rows, a.visualRow, a.visualColumn); got != selection || a.visualMode != "visual" {
					t.Fatalf("width %d selection changed: %+v -> %+v", width, selection, got)
				}
			}
		})
	}
}

func TestTableAndSummaryDecorationsDoNotMoveOnRedraw(t *testing.T) {
	a := fixture(t)
	a.Session.Summary = "important context\nsecond line"
	a.Session.Messages = []session.Message{{Role: "assistant", Content: "| first | second |\n| --- | --- |\n| alpha | bravo |\nafter"}}
	for _, width := range []int{12, 80} {
		rows := a.transcript(width)
		for i, row := range rows {
			for column := 0; column < ansi.StringWidth(row.Text); column++ {
				a.chatCursor, a.chatColumn, a.follow = i, column, false
				a.transcript(width)
				if a.chatCursor != i || a.chatColumn != column {
					t.Fatalf("width %d cursor moved on unchanged redraw: %d:%d -> %d:%d", width, i, column, a.chatCursor, a.chatColumn)
				}
			}
		}
	}
}

func TestHeaderOnlyTableRetainsSecondFieldAcrossWidths(t *testing.T) {
	a := fixture(t)
	a.Session.Messages = []session.Message{{Role: "assistant", Content: "| long first header | BRAVO second header |\n| --- | --- |"}}
	for i, row := range a.transcript(100) {
		plain := ansi.Strip(row.Text)
		if column := strings.Index(plain, "BRAVO"); column >= 0 {
			a.chatCursor, a.chatColumn = i, column
		}
	}
	want := anchorAt(a.rows, a.chatCursor, a.chatColumn)
	for _, width := range []int{12, 12, 100, 8, 8, 100} {
		a.transcript(width)
		if got := anchorAt(a.rows, a.chatCursor, a.chatColumn); got != want {
			t.Fatalf("width %d header anchor changed: %+v -> %+v", width, want, got)
		}
	}
}
