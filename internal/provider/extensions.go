package provider

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/session"
)

// Adapter supplies a transport while the client retains privacy and action validation.
type Adapter interface {
	Models(context.Context) ([]ollama.Model, error)
	Show(context.Context, string) (ollama.Info, error)
	Chat(context.Context, ollama.Request, func(ollama.Response)) (ollama.Response, error)
}

// PromptMessage keeps protected evidence behind an immutable request-local reference.
type PromptMessage struct {
	Role      string `json:"role"`
	Content   string `json:"content,omitempty"`
	Reference string `json:"reference,omitempty"`
}

// PromptData contains only fields that a request hook may replace.
type PromptData struct {
	Model    string          `json:"model"`
	Messages []PromptMessage `json:"messages"`
	Think    any             `json:"think,omitempty"`
	Options  ollama.Options  `json:"options"`
}

// transformRequest restores evidence and flow controls after a detached prompt transformation.
func (c *Client) transformRequest(ctx context.Context, request ollama.Request) (ollama.Request, error) {
	if c.Transform == nil {
		return request, nil
	}
	data := PromptData{Model: request.Model, Think: request.Think, Options: request.Options}
	evidence := make(map[string]session.Message)
	for i, message := range request.Messages {
		entry := PromptMessage{Role: message.Role, Content: message.Content}
		if message.Evidence != nil {
			entry.Reference = "evidence:" + strconv.Itoa(i)
			entry.Content = ""
			evidence[entry.Reference] = message
		}
		data.Messages = append(data.Messages, entry)
	}

	purpose := request.CompactionTask
	if purpose == "" {
		purpose = "chat"
	}
	data, err := c.Transform(ctx, purpose, data)
	if err != nil {
		return request, fmt.Errorf("request hook: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return request, err
	}

	messages := make([]session.Message, 0, len(data.Messages))
	for _, entry := range data.Messages {
		if entry.Reference != "" {
			message, ok := evidence[entry.Reference]
			if !ok || entry.Content != "" || entry.Role != message.Role {
				return request, errors.New("request hook changed or duplicated protected evidence")
			}
			delete(evidence, entry.Reference)
			messages = append(messages, message)
			continue
		}
		if entry.Role != "system" && entry.Role != "user" && entry.Role != "assistant" {
			return request, errors.New("request hook returned an invalid message role")
		}
		messages = append(messages, session.Message{Role: entry.Role, Content: entry.Content})
	}

	request.Model, request.Messages, request.Think, request.Options = data.Model, messages, data.Think, data.Options
	return request, nil
}
