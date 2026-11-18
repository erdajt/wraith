package provider

import (
	"encoding/json"
	"errors"
	"slices"

	"github.com/erdajt/wraith/internal/contextual"
	"github.com/erdajt/wraith/internal/ollama"
)

// nativeActions selects the request transport without probing or retrying provider failures.
func (c *Client) nativeActions(request ollama.Request) (bool, error) {
	if request.CompactionTask != "" || request.Format != nil || request.ExecutionMode == "" {
		return false, nil
	}
	switch c.Profile.ActionProtocol {
	case "text":
		return false, nil
	case "", "auto":
		return c.Profile.Kind == "ollama" && slices.Contains(request.Capabilities, "tools"), nil
	case "native":
		if c.Profile.Kind == "ollama" || c.Profile.Kind == "openai" || c.Profile.Kind == "anthropic" {
			return true, nil
		}
	}
	return false, errors.New("provider does not support the configured action protocol")
}

// requestFits includes native tool schemas and checks the output reservation without integer overflow.
func requestFits(request ollama.Request) bool {
	if request.Options.Context <= 0 {
		return true
	}
	input := contextual.Estimate(request.Messages)
	if len(request.Tools) > 0 {
		data, err := json.Marshal(request.Tools)
		if err != nil {
			return false
		}
		input += (len(data)+2)/3 + 32
	}
	capacity := request.Options.Context
	return capacity >= 128 && input <= capacity-128 && request.Options.Predict >= 0 && request.Options.Predict <= capacity-128-input
}
