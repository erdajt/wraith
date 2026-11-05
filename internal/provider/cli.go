package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/session"
)

func CLI(ctx context.Context, cfg config.Provider, req ollama.Request, emit func(ollama.Response)) (ollama.Response, error) {
	result := ollama.Response{Message: session.Message{Role: "assistant"}}
	if len(cfg.Command) == 0 {
		return result, errors.New("cli command is empty")
	}
	dir, err := os.MkdirTemp("", "wraith-provider-")
	if err != nil {
		return result, err
	}
	defer os.RemoveAll(dir)
	var system []string
	var messages []session.Message
	for _, m := range req.Messages {
		if m.Role == "system" {
			system = append(system, m.Content)
		} else {
			messages = append(messages, session.Message{Role: m.Role, Content: m.Content})
		}
	}
	if req.Format != nil {
		system = append(system, "Return one valid JSON object without markdown fences.")
	}
	instructions := strings.Join(system, "\n\n")
	argv := append([]string(nil), cfg.Command...)
	if cfg.Kind == "codex-cli" {
		argv = append(argv, "exec", "--json", "--ephemeral", "--skip-git-repo-check", "--ignore-user-config", "--sandbox", "read-only", "--disable", "shell_tool", "-c", "web_search=\"disabled\"", "-c", "apps._default.enabled=false", "-c", "developer_instructions="+strconv.Quote(instructions))
		if req.Model != "" && req.Model != "default" {
			argv = append(argv, "--model", req.Model)
		}
		if effort, ok := req.Think.(string); ok {
			argv = append(argv, "-c", "model_reasoning_effort="+strconv.Quote(effort))
		}
		argv = append(argv, "-")
	} else {
		path := filepath.Join(dir, "system.txt")
		if err := os.WriteFile(path, []byte(instructions), 0600); err != nil {
			return result, err
		}
		argv = append(argv, "-p", "--output-format", "stream-json", "--verbose", "--include-partial-messages", "--safe-mode", "--tools", "", "--disallowedTools", "mcp__*", "--disable-slash-commands", "--system-prompt-file", path)
		if req.Model != "" && req.Model != "default" {
			argv = append(argv, "--model", req.Model)
		}
		if effort, ok := req.Think.(string); ok {
			argv = append(argv, "--effort", effort)
		}
	}
	input, err := json.Marshal(messages)
	if err != nil {
		return result, err
	}
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(child, argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	if cfg.APIKey != "" && cfg.KeyEnv != "" {
		cmd.Env = append(cmd.Env, cfg.KeyEnv+"="+cfg.APIKey)
	}
	cmd.Stdin = io.MultiReader(strings.NewReader("Continue this supplied conversation as the assistant. Use the supplied system instructions. Conversation JSON:\n"), bytes.NewReader(input))
	cmd.Stderr = io.Discard
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = time.Second
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return result, err
	}
	start := time.Now()
	if err := cmd.Start(); err != nil {
		return result, fmt.Errorf("start provider cli: %w", err)
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64<<10), 4<<20)
	for scanner.Scan() {
		if len(bytes.TrimSpace(scanner.Bytes())) == 0 {
			continue
		}
		if err = cliEvent(cfg.Kind, scanner.Bytes(), &result, emit); err != nil {
			cancel()
			break
		}
	}
	if err == nil {
		err = scanner.Err()
	}
	if err != nil {
		cancel()
	}
	waitErr := cmd.Wait()
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	if err == nil {
		err = waitErr
	}
	if err == nil && !result.Done {
		err = errors.New("provider cli ended without a completed turn")
	}
	result.EvalDuration = time.Since(start).Nanoseconds()
	return result, err
}
