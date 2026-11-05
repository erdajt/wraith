package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/ollama"
)

func (a *App) probe() tea.Cmd {
	if a.demo {
		return nil
	}
	if a.loadCancel != nil {
		a.loadCancel()
	}
	timeout, _ := time.ParseDuration(a.Config.Timeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	a.loadCancel = cancel
	a.loadSerial++
	a.loading, a.connected = true, false
	serial, model, client, cfg := a.loadSerial, a.Session.Model, a.Client, a.Config
	logPath := filepath.Join(a.Paths.Cache, "ollama.log")
	return func() tea.Msg {
		defer cancel()
		result := infoMsg{Serial: serial, Model: model}
		if err := client.EnsureServer(ctx, cfg.AutoStart, logPath); err != nil {
			result.Err = fmt.Errorf("ollama startup: %w · log: %s", err, logPath)
			return result
		}
		result.Info, result.Err = client.Show(ctx, model)
		if result.Err == nil {
			result.Err = client.Load(ctx, model, cfg.KeepAlive, ollama.Options{Context: cfg.Context, Threads: cfg.Threads, Predict: cfg.Predict, Temperature: cfg.Temperature})
		}
		return result
	}
}
