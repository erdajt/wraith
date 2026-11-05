package ollama

import (
	"context"
	"encoding/json"
	"errors"
	"io"
)

func (c *Client) Load(ctx context.Context, model, keepAlive string, options Options) error {
	resp, err := c.do(ctx, "/api/generate", struct {
		Model     string  `json:"model"`
		Prompt    string  `json:"prompt"`
		Stream    bool    `json:"stream"`
		KeepAlive string  `json:"keep_alive"`
		Options   Options `json:"options"`
	}{Model: model, KeepAlive: keepAlive, Options: options})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var result Response
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return err
	}
	if result.Error != "" {
		return errors.New(result.Error)
	}
	if !result.Done {
		return errors.New("model load ended without a completion marker")
	}
	return nil
}
