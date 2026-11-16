package ui

import (
	"fmt"
	"slices"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/runner"
)

func (a *App) jobList() ([]runner.Record, error) {
	records, err := a.history().List()
	if err != nil {
		return nil, err
	}
	positions := map[string]int{}
	for i, r := range records {
		positions[r.ID] = i
	}
	for _, job := range a.jobs.Snapshot() {
		r := job.Record
		r.Output, r.Tail = "", ""
		if i, ok := positions[r.ID]; ok {
			records[i] = r
		} else {
			records = append(records, r)
		}
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Started.After(records[j].Started) })
	return records, nil
}

func (a *App) openJobs() tea.Cmd {
	records, err := a.jobList()
	if err != nil {
		a.notice = err.Error()
		return nil
	}
	items := []item{}
	for _, r := range records {
		if r.Session != a.Session.ID {
			continue
		}
		policy := "output private"
		if r.ShareOutput && !r.OutputHidden {
			policy = "output shared"
		}
		location := "foreground"
		if r.Background {
			location = "background"
		}
		items = append(items, item{Label: r.Command, Value: r.ID, Detail: fmt.Sprintf("%s · %s · %s · %s", r.Status, location, policy, r.ID[:8])})
	}
	a.popup = &menu{Kind: "jobs", Title: "session jobs · enter output · " + a.Config.Keys["job_cancel"] + " cancel · r refresh", Items: items}
	return nil
}

func (a *App) cancelJob(id string) {
	if a.jobs.Cancel(id) {
		a.notice = "cancelling command · output remains in history"
	} else {
		a.notice = "command has already stopped"
	}
}

func (a *App) reconcileJobs() {
	records, err := a.history().List()
	if err != nil {
		a.notice = "command reconciliation: " + err.Error()
		return
	}
	for _, record := range records {
		if record.Session != a.Session.ID || record.Finished.IsZero() || slices.Contains(a.Session.DeliveredRuns, record.ID) {
			continue
		}
		if record.Agent || record.ShareOutput && !record.OutputHidden {
			record, err = a.history().Load(record.ID)
			if err != nil {
				a.notice = "command reconciliation: " + err.Error()
				return
			}
		}
		a.pendingRuns[record.ID] = record
	}
}
