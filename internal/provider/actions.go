package provider

import (
	"encoding/json"
	"errors"
	"fmt"
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

// ContextError reports an input and output reservation that exceed the available context.
type ContextError struct{ Input, Output, Capacity int }

// Error describes the context reservation and the available recovery options.
func (e *ContextError) Error() string {
	return fmt.Sprintf("context budget: about %d input + %d output tokens exceeds %d; compact, trim the prompt, or increase context", e.Input, e.Output, e.Capacity)
}

// CheckContext validates input and output reservations without overflowing integer arithmetic.
func CheckContext(input, output, capacity int) error {
	if capacity > 0 && (capacity < 128 || input > capacity-128 || output < 0 || output > capacity-128-input) {
		return &ContextError{Input: input, Output: output, Capacity: capacity}
	}
	return nil
}

// Estimate prepares the same private request used by chat before counting messages and tool schemas.
func (c *Client) Estimate(request ollama.Request) (int, error) {
	_, prepared, err := c.privateRequest(request)
	if err != nil {
		return 0, err
	}
	return requestTokens(prepared)
}

// requestTokens accounts for the policy and schemas present in a prepared request.
func requestTokens(request ollama.Request) (int, error) {
	input := contextual.Estimate(request.Messages)
	if len(request.Tools) > 0 {
		data, err := json.Marshal(request.Tools)
		if err != nil {
			return 0, err
		}
		input += (len(data)+2)/3 + 32
	}
	return input, nil
}

// requestFits checks the prepared transport's input and output reservation.
func requestFits(request ollama.Request) bool {
	input, err := requestTokens(request)
	return err == nil && CheckContext(input, request.Options.Predict, request.Options.Context) == nil
}
