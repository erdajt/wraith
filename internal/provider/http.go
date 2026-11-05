package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/ollama"
)

type API struct {
	Config config.Provider
	HTTP   *http.Client
}

func (a *API) request(ctx context.Context, method, path string, payload any) (*http.Response, error) {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(b)
	}
	r, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(a.Config.BaseURL, "/")+path, body)
	if err != nil {
		return nil, err
	}
	r.Header.Set("Content-Type", "application/json")
	key := a.Config.APIKey
	if a.Config.KeyEnv != "" {
		if value := os.Getenv(a.Config.KeyEnv); value != "" {
			key = value
		}
	}
	if a.Config.Kind == "anthropic" {
		r.Header.Set("anthropic-version", "2023-06-01")
		if key != "" {
			r.Header.Set("x-api-key", key)
		}
	} else if key != "" {
		r.Header.Set("Authorization", "Bearer "+key)
	}
	for name, value := range a.Config.Headers {
		r.Header.Set(name, value)
	}
	client := *a.HTTP
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := client.Do(r)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		response.Body.Close()
		return nil, fmt.Errorf("provider returned http %d; check credentials, model and request settings", response.StatusCode)
	}
	return response, nil
}

func (a *API) Models(ctx context.Context) ([]ollama.Model, error) {
	if len(a.Config.Models) > 0 {
		var models []ollama.Model
		for _, name := range a.Config.Models {
			models = append(models, ollama.Model{Name: name})
		}
		return models, nil
	}
	response, err := a.request(ctx, http.MethodGet, "/models", nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(&result); err != nil {
		return nil, err
	}
	var models []ollama.Model
	for _, entry := range result.Data {
		if entry.ID != "" {
			models = append(models, ollama.Model{Name: entry.ID})
		}
	}
	if len(models) == 0 {
		return nil, errors.New("provider returned no models; configure providers.<name>.models or select a model id directly")
	}
	return models, nil
}
