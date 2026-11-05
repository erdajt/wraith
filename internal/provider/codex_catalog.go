package provider

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/erdajt/wraith/internal/fsutil"
	"github.com/erdajt/wraith/internal/ollama"
)

type codexModel struct {
	Name    string
	Context int
	Default string
	Efforts []string
}

func codexCache() ([]codexModel, error) {
	home := os.Getenv("CODEX_HOME")
	if home == "" {
		user, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		home = filepath.Join(user, ".codex")
	}
	data, err := fsutil.ReadLimit(filepath.Join(home, "models_cache.json"), 8<<20)
	if err != nil {
		return nil, err
	}
	var cache struct {
		Models []struct {
			Slug       string `json:"slug"`
			Visibility string `json:"visibility"`
			Context    int    `json:"context_window"`
			Default    string `json:"default_reasoning_level"`
			Efforts    []struct {
				Effort string `json:"effort"`
			} `json:"supported_reasoning_levels"`
		} `json:"models"`
	}
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}
	var models []codexModel
	seen := map[string]bool{}
	for _, entry := range cache.Models {
		if entry.Slug == "" || entry.Visibility == "hide" || seen[entry.Slug] {
			continue
		}
		seen[entry.Slug] = true
		model := codexModel{Name: entry.Slug, Context: entry.Context, Default: entry.Default}
		for _, effort := range entry.Efforts {
			if effort.Effort != "" {
				model.Efforts = append(model.Efforts, effort.Effort)
			}
		}
		models = append(models, model)
	}
	if len(models) == 0 {
		return nil, errors.New("codex catalog has no visible models")
	}
	return models, nil
}

func codexInfo(models []codexModel, model string, info ollama.Info) ollama.Info {
	for _, entry := range models {
		if entry.Name != model {
			continue
		}
		info.ContextWindow = entry.Context
		if len(info.Thinking.Values) == 0 {
			for _, effort := range entry.Efforts {
				info.Thinking.Values = append(info.Thinking.Values, effort)
			}
			info.Thinking.Default = entry.Default
		}
		break
	}
	return info
}
