package ui

import (
	"encoding/json"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/ollama"
)

func (a *App) set(key, value string) tea.Cmd {
	if a.demo {
		a.notice = "settings are read-only in demo mode"
		return nil
	}
	b, _ := json.Marshal(a.Config)
	var data map[string]any
	_ = json.Unmarshal(b, &data)
	var decoded any
	if err := json.Unmarshal([]byte(value), &decoded); err != nil {
		decoded = value
	}
	parent := data
	parts := strings.Split(key, ".")
	for _, part := range parts[:len(parts)-1] {
		child, ok := parent[part].(map[string]any)
		if !ok {
			a.notice = "unknown setting: " + key
			return nil
		}
		parent = child
	}
	leaf := parts[len(parts)-1]
	if _, ok := parent[leaf]; !ok {
		a.notice = "unknown setting: " + key
		return nil
	}
	parent[leaf] = decoded
	b, _ = json.Marshal(data)
	var c config.Config
	if err := json.Unmarshal(b, &c); err != nil {
		a.notice = err.Error()
		return nil
	}
	if key == "model" {
		c.Thinking = "auto"
	}
	if err := c.Validate(); err != nil {
		a.notice = err.Error()
		return nil
	}
	if key == "thinking" {
		if _, err := a.Info.Think(c.Thinking); err != nil {
			a.notice = err.Error()
			return nil
		}
	}
	if err := config.Save(config.File(a.Paths.Config), c); err != nil {
		a.notice = err.Error()
		return nil
	}
	a.Config = c
	if key == "appearance.thinking" {
		a.showThinking = c.Appearance.Thinking
		a.thinkingExpanded = map[string]bool{}
	}
	if key == "appearance.mouse" {
		if c.Appearance.Mouse {
			return tea.EnableMouseCellMotion
		}
		return tea.DisableMouse
	}
	a.resizePanes()
	a.notice = "saved " + key + " = " + value
	if key == "host" {
		a.Client = ollama.New(c.Host)
		a.connected = false
		return a.probe()
	}
	if key == "model" || key == "context" || key == "threads" {
		if key == "model" {
			a.Info = ollama.Info{}
		}
		a.Session.Model = c.Model
		a.persist("model changed")
		return a.probe()
	}
	return nil
}

func (a *App) settingMenu(key string) tea.Cmd {
	if strings.HasPrefix(key, "category.") {
		a.settingsCategory(strings.TrimPrefix(key, "category."))
		return nil
	}
	if key == "edit" {
		return a.edit("config", "", "", -1)
	}
	if key == "model" {
		return a.modelCommand("models", "")
	}
	if key == "theme" {
		return a.runCommand("theme")
	}
	if key == "thinking" {
		return a.runCommand("think")
	}
	values := map[string][]string{"execution.auto_share": {"false", "true"}, "context": {"4096", "8192", "16384", "32768", "65536"}, "predict": {"512", "1024", "2048", "4096"}, "compact.mode": {"ask", "auto", "manual"}, "compact.threshold": {"0.7", "0.8", "0.9", "0.95"}, "terminal.background": {"inherit", "theme"}, "appearance.background": {"theme", "inherit"}, "appearance.mouse": {"true", "false"}, "appearance.thinking": {"true", "false"}, "animations": {"true", "false"}, "execution.mode": {"agent", "blocks"}, "execution.max_steps": {"1", "4", "8", "16"}, "execution.output_context": {"1024", "4096", "8192", "16384"}, "auto_start": {"true", "false"}, "threads": {"0", "4", "6", "8"}, "terminal.overlay_width": {"0.4", "0.5", "0.6", "0.7", "0.8", "0.9", "0.95"}, "terminal.overlay_height": {"0.3", "0.4", "0.5", "0.6", "0.7", "0.8", "0.9"}, "terminal.split_width": {"0.25", "0.3", "0.4", "0.5", "0.6", "0.7"}}
	if len(values[key]) == 0 {
		return a.edit("config", "", "", -1)
	}
	items := []item{}
	if key == "context" {
		items = append(items, item{"custom capacity", "context=custom", "any positive integer"})
	}
	for _, v := range values[key] {
		items = append(items, item{v, key + "=" + v, ""})
	}
	a.popup = &menu{Kind: "value", Title: key, Items: items}
	return nil
}
