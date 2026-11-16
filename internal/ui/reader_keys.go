package ui

import tea "github.com/charmbracelet/bubbletea"

func (a *App) readerKey(k tea.KeyMsg) tea.Cmd {
	m := a.popup
	if k.String() == a.Config.Keys["job_cancel"] {
		a.cancelJob(m.Record.ID)
		return nil
	}
	if k.String() == a.Config.Keys["output_share"] && (!m.Record.ShareOutput || m.Record.OutputHidden) {
		return a.shareRun(m.Record)
	}
	if k.String() == "r" && m.Reader.Buffer.Pending == "" && m.Reader.Buffer.Mode == "normal" {
		m.Reader.Frozen = false
		a.showRun(m.Record, m.Record.Output)
		return nil
	}
	text, exit := m.Reader.key(k, min(a.height-8, 18)-3)
	if exit {
		a.popup = nil
	}
	if text != "" {
		return a.copyText(text, m.Reader.Buffer.Linewise)
	}
	return nil
}
