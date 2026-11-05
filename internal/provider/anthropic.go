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

func (a *API) Anthropic(ctx context.Context, req ollama.Request, emit func(ollama.Response)) (ollama.Response, error) {
	var system []string
	var messages []map[string]string
	for _, m := range req.Messages {
		if m.Role == "system" {
			system = append(system, m.Content)
		} else {
			messages = append(messages, map[string]string{"role": m.Role, "content": m.Content})
		}
	}
	if req.Format != nil {
		system = append(system, "Return a single valid JSON object without markdown fences.")
	}
	body := map[string]any{"model": req.Model, "system": strings.Join(system, "\n\n"), "messages": messages, "max_tokens": req.Options.Predict, "stream": true}
	if req.Think == false {
		body["thinking"] = map[string]string{"type": "disabled"}
	} else if req.Think != nil {
		if a.Config.ThinkingMode == "adaptive" {
			body["thinking"] = map[string]string{"type": "adaptive"}
			if effort, ok := req.Think.(string); ok {
				body["output_config"] = map[string]string{"effort": effort}
			}
		} else {
			if req.Options.Predict <= 1024 {
				return ollama.Response{}, errors.New("anthropic thinking requires predict greater than 1024")
			}
			body["thinking"] = map[string]any{"type": "enabled", "budget_tokens": max(1024, req.Options.Predict/2)}
		}
	}
	for key, value := range a.Config.Extra {
		body[key] = value
	}
	start := time.Now()
	response, err := a.request(ctx, http.MethodPost, "/messages", body)
	result := ollama.Response{Message: session.Message{Role: "assistant"}}
	if err != nil {
		return result, err
	}
	defer response.Body.Close()
	err = readSSE(response.Body, func(_ string, data []byte) error {
		var event struct {
			Type    string `json:"type"`
			Message struct {
				Usage struct {
					Input int `json:"input_tokens"`
				} `json:"usage"`
			} `json:"message"`
			Block struct {
				Type     string `json:"type"`
				Text     string `json:"text"`
				Thinking string `json:"thinking"`
			} `json:"content_block"`
			Delta struct {
				Type     string `json:"type"`
				Text     string `json:"text"`
				Thinking string `json:"thinking"`
				Stop     string `json:"stop_reason"`
			} `json:"delta"`
			Usage struct {
				Output int `json:"output_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(data, &event); err != nil {
			return err
		}
		text, thinking := "", ""
		switch event.Type {
		case "error":
			return errors.New("anthropic reported a stream error")
		case "message_start":
			result.PromptCount = event.Message.Usage.Input
		case "content_block_start":
			if event.Block.Type == "tool_use" {
				return errors.New("anthropic returned an unsupported native tool call")
			}
			text, thinking = event.Block.Text, event.Block.Thinking
		case "content_block_delta":
			text, thinking = event.Delta.Text, event.Delta.Thinking
		case "message_delta":
			result.Reason = event.Delta.Stop
			result.EvalCount = event.Usage.Output
		case "message_stop":
			result.Done = true
		}
		result.Message.Content += text
		result.Message.Thinking += thinking
		if len(result.Message.Content)+len(result.Message.Thinking) > 16<<20 {
			return errors.New("provider response exceeds 16 mib")
		}
		if emit != nil && (text != "" || thinking != "") {
			emit(ollama.Response{Message: session.Message{Role: "assistant", Content: text, Thinking: thinking}})
		}
		return nil
	})
	result.EvalDuration = time.Since(start).Nanoseconds()
	if result.Reason == "max_tokens" {
		result.Reason = "length"
	}
	if err == nil && !result.Done {
		err = errors.New("anthropic stream ended before completion")
	}
	return result, err
}
