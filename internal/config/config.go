package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/erdajt/wraith/internal/fsutil"
)

const DefaultModel = "hf.co/filvyb/Qwen3.5-9B-heretic-v2-GGUF:Q5_K_M"

type Config struct {
	Version     int               `json:"version"`
	Host        string            `json:"host"`
	Model       string            `json:"model"`
	Context     int               `json:"context"`
	Predict     int               `json:"predict"`
	Temperature float64           `json:"temperature"`
	Thinking    string            `json:"thinking"`
	Theme       string            `json:"theme"`
	Editor      []string          `json:"editor"`
	Terminal    Terminal          `json:"terminal"`
	Shell       []string          `json:"shell"`
	KeepAlive   string            `json:"keep_alive"`
	Timeout     string            `json:"timeout"`
	Compact     Compact           `json:"compact"`
	Keys        map[string]string `json:"keys"`
	Animations  bool              `json:"animations"`
}

type Compact struct {
	Mode      string  `json:"mode"`
	Threshold float64 `json:"threshold"`
	KeepTurns int     `json:"keep_turns"`
	Budget    int     `json:"budget"`
}

type Terminal struct {
	Background    string  `json:"background"`
	Foreground    string  `json:"foreground"`
	OverlayWidth  float64 `json:"overlay_width"`
	OverlayHeight float64 `json:"overlay_height"`
	SplitWidth    float64 `json:"split_width"`
	Scrollback    int     `json:"scrollback"`
	Directory     string  `json:"directory"`
}

type Paths struct{ Config, Data, Cache string }

func Default() Config {
	return Config{Version: 1, Host: "http://127.0.0.1:11434", Model: DefaultModel,
		Context: 8192, Predict: 2048, Temperature: 0.3, Thinking: "auto", Theme: "phosphor",
		Editor: []string{"nvim"}, Shell: []string{"zsh", "-i"}, KeepAlive: "2m", Timeout: "10m",
		Terminal: Terminal{Background: "inherit", Foreground: "inherit", OverlayWidth: 0.7, OverlayHeight: 0.6, SplitWidth: 0.4, Scrollback: 5000},
		Compact:  Compact{Mode: "ask", Threshold: 0.9, KeepTurns: 2, Budget: 1536},
		Keys:     map[string]string{"send": "ctrl+s", "editor": "ctrl+e", "focus": "tab", "cancel": "ctrl+c", "quit": "ctrl+q", "terminal_overlay": "alt+e", "terminal_split": "alt+s", "terminal_unfocus": "ctrl+\\"}, Animations: true}
}

func Locations() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, err
	}
	get := func(key, fallback string) string {
		if s := os.Getenv(key); filepath.IsAbs(s) {
			return filepath.Join(s, "wraith")
		}
		return filepath.Join(home, fallback, "wraith")
	}
	return Paths{get("XDG_CONFIG_HOME", ".config"), get("XDG_DATA_HOME", ".local/share"), get("XDG_CACHE_HOME", ".cache")}, nil
}

func Load(path string) (Config, error) {
	c := Default()
	b, err := fsutil.ReadLimit(path, 1<<20)
	if errors.Is(err, os.ErrNotExist) {
		return c, Save(path, c)
	}
	if err != nil {
		return c, err
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&c); err != nil {
		return c, fmt.Errorf("config: %w", err)
	}
	var extra any
	if d.Decode(&extra) == nil {
		return c, errors.New("config has trailing data")
	}
	return c, c.Validate()
}

func Save(path string, c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.Bytes(path, append(b, '\n'))
}

func (c Config) Validate() error {
	if c.Version != 1 {
		return errors.New("unsupported config version")
	}
	u, err := url.Parse(c.Host)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("host must be an http(s) url without credentials, query or fragment")
	}
	if strings.TrimSpace(c.Model) == "" {
		return errors.New("model is empty")
	}
	if c.Context < 2048 || c.Context > 262144 {
		return errors.New("context must be between 2048 and 262144")
	}
	if c.Predict < 128 || c.Predict > c.Context/2 {
		return errors.New("predict must be between 128 and half the context")
	}
	if c.Temperature < 0 || c.Temperature > 2 {
		return errors.New("temperature must be between 0 and 2")
	}
	if !slices.Contains([]string{"auto", "off", "on", "low", "medium", "high"}, c.Thinking) {
		return errors.New("thinking must be auto, off, on, low, medium or high")
	}
	if !slices.Contains([]string{"phosphor", "tokyo", "ember", "ice", "mono"}, c.Theme) {
		return errors.New("unknown theme")
	}
	for _, argv := range [][]string{c.Editor, c.Shell} {
		if len(argv) == 0 || strings.TrimSpace(argv[0]) == "" {
			return errors.New("editor and shell must be nonempty argument arrays")
		}
	}
	for _, argv := range [][]string{c.Editor, c.Shell} {
		for _, arg := range argv {
			if strings.ContainsRune(arg, 0) {
				return errors.New("command argument contains a null byte")
			}
		}
	}
	if c.Terminal.OverlayWidth < 0.3 || c.Terminal.OverlayWidth > 0.95 || c.Terminal.OverlayHeight < 0.3 || c.Terminal.OverlayHeight > 0.95 || c.Terminal.SplitWidth < 0.25 || c.Terminal.SplitWidth > 0.7 {
		return errors.New("terminal pane proportions are out of range")
	}
	if c.Terminal.Scrollback < 0 || c.Terminal.Scrollback > 100000 {
		return errors.New("terminal scrollback must be between 0 and 100000")
	}
	for _, color := range []string{c.Terminal.Background, c.Terminal.Foreground} {
		if color != "inherit" && color != "theme" && !validColor(color) {
			return errors.New("terminal colors must be inherit, theme or #rrggbb")
		}
	}
	for _, d := range []string{c.KeepAlive, c.Timeout} {
		v, e := time.ParseDuration(d)
		if e != nil || v <= 0 {
			return errors.New("keep_alive and timeout must be positive durations")
		}
	}
	if !slices.Contains([]string{"ask", "auto", "manual"}, c.Compact.Mode) {
		return errors.New("compact.mode must be ask, auto or manual")
	}
	if c.Compact.Threshold < 0.5 || c.Compact.Threshold > 0.95 {
		return errors.New("compact.threshold must be between 0.5 and 0.95")
	}
	if c.Compact.KeepTurns < 1 || c.Compact.KeepTurns > 20 {
		return errors.New("compact.keep_turns must be between 1 and 20")
	}
	if c.Compact.Budget < 256 || c.Compact.Budget > c.Context/2 {
		return errors.New("compact.budget must be between 256 and half the context")
	}
	seen := map[string]bool{}
	for _, key := range []string{"send", "editor", "focus", "cancel", "quit", "terminal_overlay", "terminal_split", "terminal_unfocus"} {
		v := c.Keys[key]
		if v == "" || seen[v] {
			return fmt.Errorf("missing or duplicate key binding: %s", key)
		}
		seen[v] = true
	}
	return nil
}

func validColor(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for _, r := range s[1:] {
		if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			return false
		}
	}
	return true
}
