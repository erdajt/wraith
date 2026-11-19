package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/plugin"
)

type luaProvider struct {
	runtime    *plugin.Runtime
	definition plugin.ProviderDefinition
}

type providerDelta struct {
	Content      string         `json:"content,omitempty"`
	Thinking     string         `json:"thinking,omitempty"`
	Calls        []providerCall `json:"calls,omitempty"`
	Reason       string         `json:"reason,omitempty"`
	PromptTokens int            `json:"prompt_tokens,omitempty"`
	OutputTokens int            `json:"output_tokens,omitempty"`
	Duration     int64          `json:"duration_ns,omitempty"`
}

type providerCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// begin freezes the accepted implementation and lets the ui dispatch its asynchronous callback.
func (p *luaProvider) begin(ctx context.Context, method string, data any) (*plugin.Invocation, error) {
	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	d := p.definition
	callback := map[string]string{"models": d.Models, "show": d.Show, "chat": d.Chat}[method]
	return p.runtime.BeginInvocation(ctx, plugin.InvocationRequest{
		Binding: plugin.InvocationBinding{Kind: "provider", Name: d.Name, Method: method, Owner: d.Owner, Lease: d.Lease, Callback: callback, Revision: d.Revision},
		Data:    encoded,
	})
}

// decodeProviderValue rejects unknown wire fields and trailing values before native use.
func decodeProviderValue(data []byte, target any) error {
	data = bytes.TrimSpace(data)
	expected := byte('{')
	if _, models := target.(*[]ollama.Model); models {
		expected = '['
	}
	if len(data) == 0 || data[0] != expected {
		return errors.New("extension provider response has the wrong container type")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("extension provider response: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("extension provider response contains trailing data")
	}
	return nil
}

// metadata accepts one terminal discovery result; discovery methods do not stream.
func (p *luaProvider) metadata(ctx context.Context, method string, request, target any) error {
	invocation, err := p.begin(ctx, method, request)
	if err != nil {
		return err
	}
	defer invocation.Close()
	event, err := invocation.Next(ctx)
	if err != nil {
		return err
	}
	if event.Error != "" {
		return errors.New(event.Error)
	}
	if !event.Final {
		return errors.New("extension provider discovery must finish without streaming")
	}
	return decodeProviderValue(event.Data, target)
}

// Models validates model identifiers before exposing them in the native picker.
func (p *luaProvider) Models(ctx context.Context) ([]ollama.Model, error) {
	var models []ollama.Model
	if err := p.metadata(ctx, "models", map[string]any{}, &models); err != nil {
		return nil, err
	}
	if len(models) > 1024 {
		return nil, errors.New("extension provider returned more than 1024 models")
	}
	seen := map[string]bool{}
	for _, model := range models {
		if strings.TrimSpace(model.Name) == "" || len(model.Name) > 512 || strings.ContainsAny(model.Name, "\x00\r\n\x1b") || model.Size < 0 || seen[model.Name] {
			return nil, errors.New("extension provider returned an invalid or repeated model")
		}
		seen[model.Name] = true
	}
	return models, nil
}

// Show checks discovery bounds before model defaults enter configuration or context accounting.
func (p *luaProvider) Show(ctx context.Context, model string) (ollama.Info, error) {
	var info ollama.Info
	if err := p.metadata(ctx, "show", map[string]string{"model": model}, &info); err != nil {
		return info, err
	}
	if info.ContextWindow < 0 || len(info.Capabilities) > 32 || len(info.Thinking.Values) > 32 {
		return ollama.Info{}, errors.New("extension provider returned invalid model limits")
	}
	for _, capability := range info.Capabilities {
		if len(capability) > 128 || strings.ContainsAny(capability, "\x00\r\n\x1b") {
			return ollama.Info{}, errors.New("extension provider returned an invalid capability")
		}
	}
	for _, value := range append(info.Thinking.Values, info.Thinking.Default) {
		switch value := value.(type) {
		case nil, bool:
		case string:
			if len(value) > 128 {
				return ollama.Info{}, errors.New("extension provider thinking value exceeds 128 bytes")
			}
		default:
			return ollama.Info{}, errors.New("extension provider thinking values must be boolean or text")
		}
	}
	return info, nil
}

// Chat forwards ordered deltas while retaining native privacy, repair and action normalization.
func (p *luaProvider) Chat(ctx context.Context, request ollama.Request, emit func(ollama.Response)) (result ollama.Response, err error) {
	invocation, err := p.begin(ctx, "chat", request)
	if err != nil {
		return result, err
	}
	defer invocation.Close()
	var content, thinking strings.Builder
	defer func() {
		result.Message.Role = "assistant"
		result.Message.Content, result.Message.Thinking = content.String(), thinking.String()
	}()
	bytesSeen := 0
	for {
		event, err := invocation.Next(ctx)
		if err != nil {
			return result, err
		}
		if event.Error != "" {
			return result, errors.New(event.Error)
		}
		var delta providerDelta
		if err := decodeProviderValue(event.Data, &delta); err != nil {
			return result, err
		}
		bytesSeen += len(delta.Content) + len(delta.Thinking)
		if bytesSeen > 16<<20 {
			return result, errors.New("extension provider response exceeds 16 mib")
		}
		if !event.Final && (len(delta.Calls) > 0 || delta.Reason != "" || delta.PromptTokens != 0 || delta.OutputTokens != 0 || delta.Duration != 0) {
			return result, errors.New("extension provider deltas may contain only content and thinking")
		}
		if delta.PromptTokens < 0 || delta.OutputTokens < 0 || delta.Duration < 0 || len(delta.Calls) > 1 || delta.Reason != "" && delta.Reason != "stop" && delta.Reason != "length" {
			return result, errors.New("extension provider returned invalid completion metadata")
		}
		response := ollama.Response{Done: event.Final, Reason: delta.Reason, PromptCount: delta.PromptTokens, EvalCount: delta.OutputTokens, EvalDuration: delta.Duration}
		response.Message.Role, response.Message.Content, response.Message.Thinking = "assistant", delta.Content, delta.Thinking
		for _, call := range delta.Calls {
			if strings.TrimSpace(call.Name) == "" || len(call.Name) > 128 || len(call.Arguments) > 32<<10 || len(call.Arguments) == 0 || bytes.TrimSpace(call.Arguments)[0] != '{' || delta.Reason == "length" {
				return result, errors.New("extension provider returned an invalid native call")
			}
			response.Calls = append(response.Calls, ollama.Call{Name: call.Name, Arguments: call.Arguments})
		}
		if event.Final && response.Reason == "" {
			response.Reason = "stop"
		}
		content.WriteString(delta.Content)
		thinking.WriteString(delta.Thinking)
		if emit != nil {
			chunk := response
			chunk.Calls = nil
			emit(chunk)
		}
		if event.Final {
			return response, nil
		}
	}
}
