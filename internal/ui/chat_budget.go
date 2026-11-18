package ui

import (
	"math"

	"github.com/erdajt/wraith/internal/contextual"
	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/provider"
	"github.com/erdajt/wraith/internal/session"
)

// chatRequest assembles one transport-independent request for budgeting and generation.
func (a *App) chatRequest(s session.Session, think any) ollama.Request {
	return ollama.Request{
		RunIDs: s.RunIDs(), ExecutionMode: a.Config.Execution.Mode, PrivateInventory: true,
		Model: s.Model, Messages: contextual.ForChat(s, a.Config, a.Instructions),
		Think: think, KeepAlive: a.Config.KeepAlive, Capabilities: append([]string(nil), a.Info.Capabilities...),
		Options: ollama.Options{Context: a.contextLimit(), Predict: a.Config.Predict, Temperature: a.Config.Temperature, Threads: a.Config.Threads},
	}
}

// chatBudget checks the prepared request while retaining conservative observed-token calibration.
func (a *App) chatBudget(s session.Session) (int, error) {
	input, err := a.Client.Estimate(a.chatRequest(s, nil))
	if err != nil {
		return 0, err
	}
	if s.Usage.InputEstimate > 0 && s.Usage.Prompt > s.Usage.InputEstimate {
		input = int(math.Ceil(float64(input) * float64(s.Usage.Prompt) / float64(s.Usage.InputEstimate)))
	}
	return input, provider.CheckContext(input, a.Config.Predict, a.contextLimit())
}

// chatUsed returns prepared usage for status displays, falling back only when request preparation fails.
func (a *App) chatUsed() int {
	used, err := a.chatBudget(a.Session)
	if used == 0 && err != nil {
		return contextual.ChatUsed(a.Session, a.runtimeConfig(), a.chatInstructions())
	}
	return used
}
