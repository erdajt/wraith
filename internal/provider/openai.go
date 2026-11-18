package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/session"
)

// OpenAI streams a chat-completions response and accumulates content, thinking and usage.
func (a *API) OpenAI(ctx context.Context, req ollama.Request, emit func(ollama.Response)) (ollama.Response, error) {
	messages := make([]map[string]string, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, map[string]string{"role": m.Role, "content": m.Content})
	}

	body := map[string]any{"model": req.Model, "messages": messages, "stream": true}
	tokens := a.Config.TokenField
	if tokens == "" {
		tokens = "max_tokens"
	}
	body[tokens] = req.Options.Predict
	if len(req.Tools) > 0 {
		body["tools"], body["parallel_tool_calls"] = req.Tools, false
	}

	if req.Format != nil {
		body["response_format"] = map[string]string{"type": "json_object"}
	}

	if a.Config.ThinkingMode == "reasoning_effort" && req.Think != nil {
		effort := req.Think
		if enabled, ok := effort.(bool); ok {
			effort = "none"
			if enabled {
				effort = "medium"
			}
		}
		body["reasoning_effort"] = effort
	}

	if a.Config.ThinkingMode == "thinking" && req.Think != nil {
		mode := "enabled"
		if req.Think == false {
			mode = "disabled"
		}
		body["thinking"] = map[string]string{"type": mode}
		if effort, ok := req.Think.(string); ok {
			body["reasoning_effort"] = effort
		}
	}

	for key, value := range a.Config.Extra {
		body[key] = value
	}

	start := time.Now()
	response, err := a.request(ctx, http.MethodPost, "/chat/completions", body)
	result := ollama.Response{Message: session.Message{Role: "assistant"}}
	if err != nil {
		return result, err
	}
	defer response.Body.Close()

	var content, thinking strings.Builder
	var call ollama.Call
	callID := ""
	called := false
	err = readSSE(response.Body, func(_ string, data []byte) error {
		if string(data) == "[DONE]" {
			result.Done = result.Reason != ""
			return nil
		}

		var chunk struct {
			Choices []struct {
				Index int `json:"index"`
				Delta struct {
					Content   string `json:"content"`
					Reasoning string `json:"reasoning_content"`
					Tools     []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Type     string `json:"type"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
				Finish string `json:"finish_reason"`
			} `json:"choices"`
			Usage struct {
				Input  int `json:"prompt_tokens"`
				Output int `json:"completion_tokens"`
			} `json:"usage"`
			Error json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal(data, &chunk); err != nil {
			return err
		}
		if len(chunk.Error) > 0 && string(chunk.Error) != "null" {
			return errors.New("provider reported a stream error")
		}

		if chunk.Usage.Input > 0 {
			result.PromptCount = chunk.Usage.Input
		}
		if chunk.Usage.Output > 0 {
			result.EvalCount = chunk.Usage.Output
		}

		for _, choice := range chunk.Choices {
			for _, delta := range choice.Delta.Tools {
				if len(req.Tools) == 0 || choice.Index != 0 || delta.Index != 0 || result.Reason != "" || result.Done || delta.Type != "" && delta.Type != "function" {
					return &ollama.ActionError{Problem: "native request is unavailable, duplicated or out of order"}
				}
				if delta.ID != "" {
					if callID != "" && callID != delta.ID {
						return &ollama.ActionError{Problem: "native request changed its call id"}
					}
					callID = delta.ID
				}
				called = true
				call.Name += delta.Function.Name
				call.Arguments = append(call.Arguments, delta.Function.Arguments...)
				if len(call.Name) > 128 || len(call.Arguments) > 32768 || len(callID) > 256 {
					return &ollama.ActionError{Problem: "native request exceeds its size limit"}
				}
			}

			content.WriteString(choice.Delta.Content)
			thinking.WriteString(choice.Delta.Reasoning)
			if content.Len()+thinking.Len() > 16<<20 {
				return errors.New("provider response exceeds 16 mib")
			}

			if emit != nil {
				emit(ollama.Response{Message: session.Message{Role: "assistant", Content: choice.Delta.Content, Thinking: choice.Delta.Reasoning}})
			}

			if choice.Finish != "" {
				result.Reason = choice.Finish
			}
		}
		return nil
	})
	result.Message.Content, result.Message.Thinking = content.String(), thinking.String()
	result.EvalDuration = time.Since(start).Nanoseconds()
	if err == nil && !result.Done {
		err = errors.New("provider stream ended before completion")
	}
	if err == nil && called {
		if result.Reason != "tool_calls" || callID == "" {
			err = &ollama.ActionError{Problem: "native request ended without a successful tool completion"}
		} else {
			result.Calls = []ollama.Call{call}
		}
	}
	return result, err
}
