package provider

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"syscall"
	"time"

	"github.com/erdajt/wraith/internal/config"
)

// codexCatalog uses the cached catalog or queries the codex app server.
func codexCatalog(ctx context.Context, profile config.Provider) ([]codexModel, error) {
	if models, err := codexCache(); err == nil {
		return models, nil
	}

	models, err := codexRPC(ctx, profile.Command, profile.Environment)
	if err != nil {
		return nil, fmt.Errorf("codex model discovery: %w; run codex to refresh its catalog or configure the provider models list", err)
	}
	return models, nil
}

// codexRPC retrieves visible models from a temporary codex app-server process.
func codexRPC(ctx context.Context, command, environment []string) ([]codexModel, error) {
	if len(command) == 0 {
		return nil, errors.New("cli command is empty")
	}

	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	argv := append(append([]string(nil), command[1:]...), "app-server", "--listen", "stdio://")
	cmd := exec.CommandContext(ctx, command[0], argv...)
	cmd.Env = environment
	cmd.Stderr = io.Discard
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = time.Second
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	defer stdin.Close()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	defer func() { cancel(); _ = cmd.Wait() }()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64<<10), 4<<20)
	encoder := json.NewEncoder(stdin)
	call := func(id int, method string, params any, result any) error {
		if err := encoder.Encode(map[string]any{"id": id, "method": method, "params": params}); err != nil {
			return err
		}

		for scanner.Scan() {
			var reply struct {
				ID     int             `json:"id"`
				Result json.RawMessage `json:"result"`
				Error  *struct {
					Code int `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(scanner.Bytes(), &reply); err != nil {
				return fmt.Errorf("invalid codex response: %w", err)
			}
			if reply.ID != id {
				continue
			}
			if reply.Error != nil {
				return fmt.Errorf("codex %s failed (%d)", method, reply.Error.Code)
			}
			return json.Unmarshal(reply.Result, result)
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := scanner.Err(); err != nil {
			return err
		}
		return io.ErrUnexpectedEOF
	}
	var initialized json.RawMessage
	if err := call(1, "initialize", map[string]any{"clientInfo": map[string]string{"name": "wraith", "version": "1"}}, &initialized); err != nil {
		return nil, err
	}
	if err := encoder.Encode(map[string]string{"method": "initialized"}); err != nil {
		return nil, err
	}

	var models []codexModel
	cursor := ""
	seen := map[string]bool{}

	for page := 0; page < 32; page++ {
		params := map[string]any{"includeHidden": false, "limit": 100}
		if cursor != "" {
			params["cursor"] = cursor
		}

		var result struct {
			Data []struct {
				Model   string `json:"model"`
				Hidden  bool   `json:"hidden"`
				Default string `json:"defaultReasoningEffort"`
				Efforts []struct {
					Effort string `json:"reasoningEffort"`
				} `json:"supportedReasoningEfforts"`
			} `json:"data"`
			Next string `json:"nextCursor"`
		}
		if err := call(page+2, "model/list", params, &result); err != nil {
			return nil, err
		}

		for _, entry := range result.Data {
			if entry.Hidden || entry.Model == "" || seen[entry.Model] {
				continue
			}
			seen[entry.Model] = true
			model := codexModel{Name: entry.Model, Default: entry.Default}
			for _, effort := range entry.Efforts {
				model.Efforts = append(model.Efforts, effort.Effort)
			}
			models = append(models, model)
		}

		if result.Next == "" {
			if len(models) == 0 {
				return nil, errors.New("codex catalog has no visible models")
			}
			return models, nil
		}

		if result.Next == cursor {
			return nil, errors.New("codex model cursor did not advance")
		}
		cursor = result.Next
	}

	return nil, errors.New("codex model catalog exceeds 32 pages")
}
