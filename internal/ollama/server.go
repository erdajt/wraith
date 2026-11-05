package ollama

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func (c *Client) EnsureServer(ctx context.Context, autoStart bool, logPath string) error {
	return c.ensureServer(ctx, autoStart, func(host string) error { return launchServer(host, logPath) })
}

func (c *Client) ensureServer(ctx context.Context, autoStart bool, launch func(string) error) error {
	probe := func() error {
		check, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		_, err := c.Models(check)
		return err
	}
	err := probe()
	if err == nil {
		return nil
	}
	u, parseErr := url.Parse(c.Base)
	if !autoStart || parseErr != nil || !localServer(u) || !errors.Is(err, syscall.ECONNREFUSED) {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = launch(u.Host); err != nil {
		return fmt.Errorf("start ollama: %w", err)
	}
	deadline, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	timer := time.NewTicker(200 * time.Millisecond)
	defer timer.Stop()
	for {
		select {
		case <-deadline.Done():
			return fmt.Errorf("waiting for ollama: %w", deadline.Err())
		case <-timer.C:
			if err = probe(); err == nil {
				return nil
			}
		}
	}
}

func localServer(u *url.URL) bool {
	ip := net.ParseIP(u.Hostname())
	return u.Scheme == "http" && (u.Hostname() == "localhost" || ip != nil && ip.IsLoopback()) && (u.Path == "" || u.Path == "/")
}

func launchServer(host, logPath string) error {
	if err := os.MkdirAll(filepath.Dir(logPath), 0700); err != nil {
		return err
	}
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer log.Close()
	cmd := exec.Command("ollama", "serve")
	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, "OLLAMA_HOST=") {
			cmd.Env = append(cmd.Env, env)
		}
	}
	cmd.Env = append(cmd.Env, "OLLAMA_HOST="+host)
	cmd.Stdout, cmd.Stderr = log, log
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
