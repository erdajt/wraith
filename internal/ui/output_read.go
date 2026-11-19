package ui

import (
	"context"
	"time"

	"github.com/erdajt/wraith/internal/contextual"
	"github.com/erdajt/wraith/internal/plugin"
	"github.com/erdajt/wraith/internal/runner"
	"github.com/erdajt/wraith/internal/session"
	"github.com/erdajt/wraith/internal/variables"
)

type outputReadState struct {
	pending             *contextual.ToolRequest
	request             contextual.ToolRequest
	runtime             *plugin.Runtime
	session, generation string
	mode                string
	variables           variables.Snapshot
	cancel              context.CancelFunc
	result              *outputReadMsg
}

type outputReadMsg struct {
	state    *outputReadState
	text     string
	evidence *session.Evidence
	nextLine int64
	waiting  bool
}

// readToolOutput projects detached output while holding the shared source budget.
func readToolOutput(ctx context.Context, state *outputReadState, scans *workspaceScanStore, history runner.History, record runner.Record, budget int, nextLine int64, lastPoll time.Time) outputReadMsg {
	result := outputReadMsg{state: state, text: "output tool denied: command is not in this session", nextLine: nextLine}
	err := scans.readSource(ctx, func() error {
		if record.ID == "" {
			var err error
			record, err = history.LoadBounded(ctx, state.request.ID, 64<<20)
			if err != nil {
				return err
			}
		}
		if record.Session != state.session {
			return nil
		}
		if !record.ShareOutput || record.OutputHidden {
			result.text = "output tool denied: output withheld by user"
			return nil
		}
		request := state.request
		if request.Tool == "poll" && record.Finished.IsZero() && time.Since(lastPoll) < time.Second {
			result.waiting = true
			result.evidence = &session.Evidence{ID: record.ID, Kind: "waiting", Status: record.Status}
			return nil
		}
		if request.Tool == "poll" {
			request.Limit, request.From = 20, max(1, nextLine)
		}
		page, err := runner.ReadOutputContext(ctx, record, request.Query, state.variables)
		if err != nil {
			return err
		}
		size := 0
		for index, line := range page.Lines {
			if len(line.Text)+40 > budget {
				page.Lines[index].Text = "[line exceeds output budget; inspect local history]"
				line = page.Lines[index]
				page.Truncated = true
			}
			size += len(line.Text) + 40
			if size > budget {
				page.Next, page.Truncated = line.Number, true
				page.Lines = page.Lines[:index]
				break
			}
		}
		if request.Tool == "poll" {
			if page.Next > 0 {
				result.nextLine = page.Next
			} else if len(page.Lines) > 0 {
				result.nextLine = page.Lines[len(page.Lines)-1].Number + 1
			}
		}
		result.evidence = &session.Evidence{ID: record.ID, Kind: "output", Status: record.Status, ExitCode: record.ExitCode, Page: &page}
		if request.Tool == "poll" && record.Finished.IsZero() && len(page.Lines) == 0 {
			result.waiting, result.evidence.Kind = true, "waiting"
		}
		return ctx.Err()
	})
	if err != nil {
		result.text = "output tool rejected: " + err.Error()
		result.evidence, result.waiting = nil, false
	}
	return result
}
