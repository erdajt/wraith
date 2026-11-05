package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/erdajt/wraith/internal/session"
)

type Options struct {
	Context     int     `json:"num_ctx"`
	Predict     int     `json:"num_predict"`
	Temperature float64 `json:"temperature"`
	Threads     int     `json:"num_thread,omitempty"`
}
type Request struct {
	Model     string            `json:"model"`
	Messages  []session.Message `json:"messages"`
	Think     any               `json:"think,omitempty"`
	Stream    bool              `json:"stream"`
	Format    any               `json:"format,omitempty"`
	KeepAlive string            `json:"keep_alive"`
	Options   Options           `json:"options"`
}
type Response struct {
	Message      session.Message `json:"message"`
	Done         bool            `json:"done"`
	Reason       string          `json:"done_reason"`
	Error        string          `json:"error"`
	PromptCount  int             `json:"prompt_eval_count"`
	EvalCount    int             `json:"eval_count"`
	EvalDuration int64           `json:"eval_duration"`
}
type Info struct {
	Capabilities []string `json:"capabilities"`
	Thinking     struct {
		Values  []any `json:"values"`
		Default any   `json:"default"`
	} `json:"thinking"`
}

type Model struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

func (c *Client) Models(ctx context.Context) ([]Model, error) {
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Base+"/api/tags", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama model list: http %d", resp.StatusCode)
	}
	var data struct {
		Models []Model `json:"models"`
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&data); err != nil {
		return nil, err
	}
	return data.Models, nil
}

type Client struct {
	Base string
	HTTP *http.Client
}

func New(base string) *Client {
	return &Client{Base: strings.TrimRight(base, "/"), HTTP: &http.Client{Timeout: 0}}
}

func (c *Client) do(ctx context.Context, path string, payload any) (*http.Response, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Base+path, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	r.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(r)
	if err != nil {
		return nil, fmt.Errorf("ollama: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		return nil, fmt.Errorf("ollama %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return resp, nil
}

func (c *Client) Show(ctx context.Context, model string) (Info, error) {
	var info Info
	resp, err := c.do(ctx, "/api/show", map[string]string{"model": model})
	if err != nil {
		return info, err
	}
	defer resp.Body.Close()
	err = json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&info)
	return info, err
}

func (c *Client) Chat(ctx context.Context, req Request, emit func(Response)) (Response, error) {
	req.Stream = true
	resp, err := c.do(ctx, "/api/chat", req)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()
	s := bufio.NewScanner(resp.Body)
	s.Buffer(make([]byte, 64<<10), 4<<20)
	var result Response
	var content, thinking strings.Builder
	for s.Scan() {
		if len(bytes.TrimSpace(s.Bytes())) == 0 {
			continue
		}
		var chunk Response
		if err = json.Unmarshal(s.Bytes(), &chunk); err != nil {
			return result, fmt.Errorf("invalid ollama stream: %w", err)
		}
		if chunk.Error != "" {
			return result, errors.New(chunk.Error)
		}
		content.WriteString(chunk.Message.Content)
		thinking.WriteString(chunk.Message.Thinking)
		if content.Len()+thinking.Len() > 16<<20 {
			return result, errors.New("response exceeds 16 mib")
		}
		if emit != nil {
			emit(chunk)
		}
		if chunk.Done {
			result = chunk
			result.Message = session.Message{Role: "assistant", Content: content.String(), Thinking: thinking.String()}
			return result, nil
		}
	}
	result.Message = session.Message{Role: "assistant", Content: content.String(), Thinking: thinking.String()}
	if err = s.Err(); err != nil {
		return result, err
	}
	return result, errors.New("ollama stream ended without a completion marker")
}

func (i Info) Think(mode string) (any, error) {
	if mode == "auto" {
		return nil, nil
	}
	var wanted any = mode
	if mode == "on" {
		wanted = true
	}
	if mode == "off" {
		wanted = false
	}
	for _, v := range i.Thinking.Values {
		if v == wanted {
			return wanted, nil
		}
	}
	if len(i.Thinking.Values) == 0 {
		for _, cap := range i.Capabilities {
			if cap == "thinking" && (mode == "on" || mode == "off") {
				return wanted, nil
			}
		}
		if mode == "off" {
			return false, nil
		}
	}
	return nil, fmt.Errorf("model does not advertise thinking=%s; use auto or a supported /think value", mode)
}

func (r Response) Usage(estimate int) session.Usage {
	u := session.Usage{Prompt: r.PromptCount, Output: r.EvalCount, InputEstimate: estimate}
	if r.EvalDuration > 0 {
		u.TokensPerSecond = float64(r.EvalCount) / time.Duration(r.EvalDuration).Seconds()
	}
	return u
}
