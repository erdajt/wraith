package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/contextual"
)

func (a *App) modelCommand(name, arg string) tea.Cmd {
	switch name {
	case "model", "models":
		client := a.Client
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			models, err := client.Models(ctx)
			sort.Slice(models, func(i, j int) bool { return models[i].Name < models[j].Name })
			return modelsMsg{Models: models, Err: err}
		}
	case "think":
		if arg != "" {
			return a.set("thinking", arg)
		}
		items := []item{{"auto · model default", "auto", ""}}
		for _, v := range a.Info.Thinking.Values {
			switch v {
			case true:
				items = append(items, item{"on", "on", ""})
			case false:
				items = append(items, item{"off", "off", ""})
			default:
				if s, ok := v.(string); ok {
					items = append(items, item{s, s, ""})
				}
			}
		}
		if len(items) == 1 {
			for _, c := range a.Info.Capabilities {
				if c == "thinking" {
					items = append(items, item{"on", "on", ""}, item{"off", "off", ""})
				}
			}
		}
		a.popup = &menu{Kind: "thinking", Title: "native model thinking", Items: items}
	case "settings":
		a.popup = &menu{Kind: "settings", Title: "settings · e opens config.json", Items: []item{{"theme", "theme", a.Config.Theme}, {"thinking", "thinking", a.Config.Thinking}, {"context window", "context", strconv.Itoa(a.Config.Context)}, {"output budget", "predict", strconv.Itoa(a.Config.Predict)}, {"compaction mode", "compact.mode", a.Config.Compact.Mode}, {"compaction threshold", "compact.threshold", fmt.Sprintf("%.0f%%", a.Config.Compact.Threshold*100)}, {"terminal background", "terminal.background", a.Config.Terminal.Background}, {"scratch width", "terminal.overlay_width", fmt.Sprintf("%.0f%%", a.Config.Terminal.OverlayWidth*100)}, {"scratch height", "terminal.overlay_height", fmt.Sprintf("%.0f%%", a.Config.Terminal.OverlayHeight*100)}, {"side shell width", "terminal.split_width", fmt.Sprintf("%.0f%%", a.Config.Terminal.SplitWidth*100)}, {"editor, shell and key bindings", "edit", "config.json"}, {"all settings", "edit", filepath.Join(a.Paths.Config, "config.json")}}}
	case "set":
		key, value, ok := strings.Cut(arg, " ")
		if !ok {
			a.notice = "usage: /set context 16384 · /set editor [\"nvim\"]"
			return nil
		}
		return a.set(key, strings.TrimSpace(value))
	case "theme":
		if arg != "" {
			return a.set("theme", arg)
		}
		a.popup = &menu{Kind: "theme", Title: "choose a theme", Items: []item{{"phosphor · mint / violet", "phosphor", ""}, {"tokyo · blue / lilac", "tokyo", ""}, {"ember · amber / rose", "ember", ""}, {"ice · cyan / silver", "ice", ""}, {"mono · grayscale", "mono", ""}}}
	case "context":
		a.notice = fmt.Sprintf("~%d / %d context tokens · output reserve %d · %d ledger facts · %d pins · %d archives · measured last prompt %d", contextual.Used(a.Session, a.Instructions), a.Config.Context, a.Config.Predict, len(a.Session.Facts), len(a.Session.Pins), len(a.Session.Archives), a.Session.Usage.Prompt)
	case "compact":
		return a.startCompact()
	case "cancel":
		if a.cancel != nil {
			a.cancel()
		}
	case "thinking":
		a.showThinking = !a.showThinking
		a.notice = fmt.Sprintf("thinking display: %t", a.showThinking)
	}
	return nil
}
