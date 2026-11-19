package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/plugin"
)

type pluginDialog struct {
	spec      plugin.Dialog
	runtime   *plugin.Runtime
	buffer    plugin.SurfaceBuffer
	reader    *reader
	matches   []int
	selected  int
	query     string
	searching bool
}

// filter retains original item indexes while rebuilding the cached generic buffer.
func (d *pluginDialog) filter() {
	d.matches = nil
	lines := []string{}
	for i, item := range d.spec.Items {
		if !strings.Contains(strings.ToLower(item.Label+" "+item.Detail), strings.ToLower(d.query)) {
			continue
		}
		d.matches = append(d.matches, i)
		line := item.Label
		if item.Detail != "" {
			line += " · " + item.Detail
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		lines = []string{"no matches"}
	}
	d.buffer.Lines = lines
	d.reader.Buffer.SyncText(strings.Join(lines, "\n"), true)
	d.selected = max(0, min(d.selected, len(d.matches)-1))
	d.selectLine()
}

// selectLine moves the visible cursor to the selected item's first rune.
func (d *pluginDialog) selectLine() {
	position := 0
	for _, line := range d.buffer.Lines[:min(d.selected, len(d.buffer.Lines))] {
		position += len([]rune(line)) + 1
	}
	d.reader.Buffer.Pos = position
}

// pluginDialogKey handles modal choices without replacing the conversation draft.
func (a *App) pluginDialogKey(k tea.KeyMsg) (command tea.Cmd) {
	d := a.pluginDialog
	defer func() {
		if a.pluginDialog == d {
			if err := a.syncPluginDialogText(d); err != nil {
				command = tea.Sequence(command, a.finishPluginDialog(map[string]any{"cancelled": true, "reason": err.Error()}))
			}
		}
	}()
	key := k.String()
	if d.spec.Kind == "input" {
		return a.pluginDialogInput(k)
	}
	if d.searching {
		switch key {
		case "enter", "up", "down":
			d.searching = false
		case "esc":
			d.searching, d.query, d.selected = false, "", 0
			d.filter()
			return nil
		case "backspace":
			runes := []rune(d.query)
			d.query = string(runes[:max(0, len(runes)-1)])
		default:
			if (k.Type == tea.KeyRunes || k.Type == tea.KeySpace) && len(d.query)+len(string(k.Runes)) <= 1024 {
				d.query += string(k.Runes)
			}
		}
		if d.searching {
			d.selected = 0
			d.filter()
			return nil
		}
	}

	switch key {
	case "esc", "q", "ctrl+c":
		return a.finishPluginDialog(map[string]any{"cancelled": true})
	case "/":
		d.searching = true
	case "j", "down":
		d.selected = min(max(0, len(d.matches)-1), d.selected+1)
	case "k", "up":
		d.selected = max(0, d.selected-1)
	case "g", "home":
		d.selected = 0
	case "G", "end":
		d.selected = max(0, len(d.matches)-1)
	case "ctrl+d", "pgdown":
		d.selected = min(max(0, len(d.matches)-1), d.selected+8)
	case "ctrl+u", "pgup":
		d.selected = max(0, d.selected-8)
	case "enter":
		if len(d.matches) > 0 {
			index := d.matches[d.selected]
			item := d.spec.Items[index]
			return a.finishPluginDialog(map[string]any{"value": item.Label, "item_id": item.ID, "index": index + 1})
		}
	}
	d.selectLine()
	return nil
}

// pluginDialogInput applies vim editing and clipboard operations to a modal input buffer.
func (a *App) pluginDialogInput(k tea.KeyMsg) tea.Cmd {
	d, key := a.pluginDialog, k.String()
	if key == "ctrl+s" || key == "enter" {
		return a.finishPluginDialog(map[string]any{"value": d.reader.Buffer.String()})
	}
	if key == "esc" && d.reader.Buffer.Mode == "normal" || key == "ctrl+c" {
		return a.finishPluginDialog(map[string]any{"cancelled": true})
	}
	before := d.reader.Buffer.String()
	d.reader.Buffer.Key(key, string(k.Runes))
	if len(d.reader.Buffer.String()) > 64<<10 {
		d.reader.Buffer.SyncText(before, true)
		a.notice = "input exceeds 64 kib"
	}
	d.buffer.Lines = strings.Split(d.reader.Buffer.String(), "\n")
	if d.reader.Buffer.Yanked {
		return a.copyText(string(d.reader.Buffer.Register), d.reader.Buffer.Linewise)
	}
	return nil
}

// pluginDialogFrame uses the generic buffer renderer for input, cursor and selection styling.
func (a *App) pluginDialogFrame() []string {
	d, p := a.pluginDialog, a.Config.Palette()
	w, h := max(4, min(a.width-6, 90)), max(1, min(a.height-8, 18))
	hints := "j/k move · / search · enter select · esc close"
	if d.spec.Kind == "input" {
		hints = "enter accept · esc normal/cancel"
	}
	content := []string{bold(p.Accent, d.spec.Title), keyHints(hints, p)}
	if d.searching || d.query != "" {
		content = append(content, ink(p.Second, "/"+clean(d.query)+"▏"))
	}
	content = append(content, renderSurfaceBuffer(d.buffer, d.reader, w-2, max(1, h-len(content)), p, true)...)
	return frame(content, w, p, false, "", "")
}
