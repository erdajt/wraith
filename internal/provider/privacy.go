package provider

import (
	"errors"
	"os"
	"strings"

	"github.com/erdajt/wraith/internal/contextual"
	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/session"
	"github.com/erdajt/wraith/internal/variables"
)

func (c *Client) privateRequest(request ollama.Request) (*Client, ollama.Request, error) {
	copy := *c
	for _, value := range []string{c.Base, c.Profile.BaseURL, request.Model, c.Profile.APIKey, os.Getenv(c.Profile.KeyEnv)} {
		if c.Variables.Contains(value) {
			return nil, request, errors.New("private variable collides with provider routing or credentials")
		}
	}
	for name, value := range c.Profile.Headers {
		if c.Variables.Contains(name) || c.Variables.Contains(value) {
			return nil, request, errors.New("private variable collides with a provider header")
		}
	}
	for _, argument := range c.Profile.Command {
		if c.Variables.Contains(argument) {
			return nil, request, errors.New("private variable collides with provider command arguments")
		}
	}
	copy.Profile.Extra = map[string]any{}
	for name, value := range c.Profile.Extra {
		if c.Variables.Contains(name) {
			return nil, request, errors.New("private variable collides with a provider option name")
		}
		sanitized, err := redactOption(value, c.Variables)
		if err != nil {
			return nil, request, err
		}
		copy.Profile.Extra[name] = sanitized
	}
	copy.Profile.Environment = []string{}
	privateNames := map[string]bool{}
	for _, entry := range c.Variables.Environment() {
		name, _, _ := strings.Cut(entry, "=")
		privateNames[name] = true
	}
	for _, entry := range os.Environ() {
		name, value, _ := strings.Cut(entry, "=")
		if privateNames[name] || c.Variables.Contains(value) {
			if name == c.Profile.KeyEnv || name == "OPENAI_API_KEY" || name == "ANTHROPIC_API_KEY" || name == "CLAUDE_CODE_OAUTH_TOKEN" {
				return nil, request, errors.New("private variable collides with provider authentication environment: " + name)
			}
			continue
		}
		copy.Profile.Environment = append(copy.Profile.Environment, entry)
	}
	request.Messages = append([]session.Message(nil), request.Messages...)
	policy := ""
	switch request.ExecutionMode {
	case "agent":
		policy = contextual.AgentPrompt
	case "blocks":
		policy = contextual.BlocksPrompt
	}
	if policy != "" && len(request.Messages) > 0 && request.Messages[0].Role == "system" {
		request.Messages[0].Content = strings.Replace(request.Messages[0].Content, "\n\n"+policy, "", 1)
	}
	compactPolicy := ""
	switch request.CompactionTask {
	case "extract":
		compactPolicy = contextual.CompactPrompt
	case "audit":
		compactPolicy = contextual.AuditPrompt
	}
	if compactPolicy != "" && len(request.Messages) > 0 && request.Messages[0].Role == "system" {
		request.Messages[0].Content = strings.TrimPrefix(request.Messages[0].Content, compactPolicy)
	}
	ids := session.EvidenceIDs(request.Messages)
	for i := range request.Messages {
		if evidence := request.Messages[i].Evidence; evidence != nil {
			request.Messages[i].Content = evidence.Render(c.Variables.Redact)
			request.Messages[i].Evidence = nil
		} else {
			request.Messages[i].Content = contextual.RedactActions(request.Messages[i].Content, c.Variables.Redact, ids)
		}
		request.Messages[i].Thinking = ""
	}

	if policy != "" {
		if len(request.Messages) > 0 && request.Messages[0].Role == "system" {
			request.Messages[0].Content += "\n\n" + policy
		} else {
			request.Messages = append([]session.Message{{Role: "system", Content: policy}}, request.Messages...)
		}
	}

	if compactPolicy != "" {
		if len(request.Messages) > 0 && request.Messages[0].Role == "system" {
			request.Messages[0].Content = compactPolicy + request.Messages[0].Content
		} else {
			request.Messages = append([]session.Message{{Role: "system", Content: compactPolicy}}, request.Messages...)
		}
	}
	if request.PrivateInventory {
		instructions, err := c.Variables.Instructions()
		if err != nil {
			return nil, request, err
		}
		if instructions != "" {
			if len(request.Messages) > 0 && request.Messages[0].Role == "system" {
				request.Messages[0].Content += instructions
			} else {
				request.Messages = append([]session.Message{{Role: "system", Content: instructions}}, request.Messages...)
			}
		}
	}
	return &copy, request, nil
}

func redactOption(value any, snapshot variables.Snapshot) (any, error) {
	switch v := value.(type) {
	case string:
		return snapshot.Redact(v), nil
	case []any:
		out := make([]any, len(v))
		for i, entry := range v {
			sanitized, err := redactOption(entry, snapshot)
			if err != nil {
				return nil, err
			}
			out[i] = sanitized
		}
		return out, nil
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, entry := range v {
			if snapshot.Contains(key) {
				return nil, errors.New("private variable collides with a provider option name")
			}
			sanitized, err := redactOption(entry, snapshot)
			if err != nil {
				return nil, err
			}
			out[key] = sanitized
		}
		return out, nil
	default:
		return v, nil
	}
}
