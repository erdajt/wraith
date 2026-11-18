package config

import "maps"

// ContextFor resolves capacity from explicit, model, provider and reported limits.
func (c Config) ContextFor(model string, reported int) int {
	p := c.Providers[c.Provider]
	if capacity := p.ModelContexts[model]; capacity > 0 {
		return capacity
	}

	if p.Context > 0 {
		return p.Context
	}

	if p.Kind == "ollama" {
		if reported > 0 {
			return min(c.Context, reported)
		}
		return c.Context
	}

	if reported > 0 {
		return reported
	}

	switch p.Kind {
	case "anthropic", "claude-cli":
		return 200000
	case "codex-cli":
		return 272000
	default:
		return 128000
	}
}

// WithModelContext copies configuration with a context override for the selected model.
func (c Config) WithModelContext(model string, capacity int) Config {
	c.Providers = maps.Clone(c.Providers)
	p := c.Providers[c.Provider]
	p.ModelContexts = maps.Clone(p.ModelContexts)
	if p.ModelContexts == nil {
		p.ModelContexts = map[string]int{}
	}

	if capacity == 0 {
		delete(p.ModelContexts, model)
	} else {
		p.ModelContexts[model] = capacity
	}

	c.Providers[c.Provider] = p
	return c
}
