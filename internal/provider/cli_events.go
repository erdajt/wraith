package provider

import (
	"encoding/json"
	"errors"

	"github.com/erdajt/wraith/internal/ollama"
)

func cliEvent(kind string, data []byte, result *ollama.Response, emit func(ollama.Response)) error {
	var event struct {
		Type   string `json:"type"`
		Error  bool   `json:"is_error"`
		Result string `json:"result"`
		Item   struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"item"`
		Event struct {
			Delta struct {
				Text     string `json:"text"`
				Thinking string `json:"thinking"`
			} `json:"delta"`
		} `json:"event"`
		Usage struct {
			Input  int `json:"input_tokens"`
			Output int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}
	chunk := ollama.Response{}
	if kind == "codex-cli" {
		switch event.Type {
		case "error", "turn.failed":
			return errors.New("codex reported a failed turn; check cli authentication and configuration")
		case "item.completed":
			switch event.Item.Type {
			case "agent_message":
				chunk.Message.Content = event.Item.Text
			case "reasoning":
				chunk.Message.Thinking = event.Item.Text
			case "command_execution", "file_change", "mcp_tool_call":
				return errors.New("cli attempted native tools; use wraith command requests instead")
			}
		case "turn.completed":
			result.Done = true
			result.Reason = "stop"
			result.PromptCount = event.Usage.Input
			result.EvalCount = event.Usage.Output
		}
	} else {
		switch event.Type {
		case "stream_event":
			chunk.Message.Content = event.Event.Delta.Text
			chunk.Message.Thinking = event.Event.Delta.Thinking
		case "result":
			if event.Error {
				return errors.New("claude reported a failed turn; check cli authentication and configuration")
			}
			if result.Message.Content == "" {
				chunk.Message.Content = event.Result
			}
			result.Done = true
			result.Reason = "stop"
			result.PromptCount = event.Usage.Input
			result.EvalCount = event.Usage.Output
		}
	}
	result.Message.Content += chunk.Message.Content
	result.Message.Thinking += chunk.Message.Thinking
	if len(result.Message.Content)+len(result.Message.Thinking) > 16<<20 {
		return errors.New("cli response exceeds 16 mib")
	}
	if emit != nil && (chunk.Message.Content != "" || chunk.Message.Thinking != "") {
		emit(chunk)
	}
	return nil
}
