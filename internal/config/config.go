package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/erdajt/wraith/internal/fsutil"
	"gopkg.in/yaml.v3"
)

const DefaultModel = "hf.co/filvyb/Qwen3.5-9B-heretic-v2-GGUF:Q5_K_M"

type Execution struct {
	AutoShare     bool   `json:"auto_share" yaml:"auto_share"`
	Mode          string `json:"mode" yaml:"mode"`
	MaxSteps      int    `json:"max_steps" yaml:"max_steps"`
	OutputContext int    `json:"output_context" yaml:"output_context"`
}

type Appearance struct {
	Background string `json:"background" yaml:"background"`
	Mouse      bool   `json:"mouse" yaml:"mouse"`
	Thinking   bool   `json:"thinking" yaml:"thinking"`
}

type Config struct {
	Provider    string              `json:"provider" yaml:"provider"`
	Providers   map[string]Provider `json:"providers" yaml:"providers"`
	Clipboard   []string            `json:"clipboard" yaml:"clipboard"`
	Appearance  Appearance          `json:"appearance" yaml:"appearance"`
	Execution   Execution           `json:"execution" yaml:"execution"`
	Themes      map[string]Palette  `json:"themes" yaml:"themes"`
	Leader      string              `json:"leader" yaml:"leader"`
	Bindings    map[string]string   `json:"bindings" yaml:"bindings"`
	AutoStart   bool                `json:"auto_start" yaml:"auto_start"`
	Version     int                 `json:"version" yaml:"version"`
	Host        string              `json:"host" yaml:"host"`
	Model       string              `json:"model" yaml:"model"`
	Context     int                 `json:"context" yaml:"context"`
	Predict     int                 `json:"predict" yaml:"predict"`
	Threads     int                 `json:"threads" yaml:"threads"`
	Temperature float64             `json:"temperature" yaml:"temperature"`
	Thinking    string              `json:"thinking" yaml:"thinking"`
	Theme       string              `json:"theme" yaml:"theme"`
	Editor      []string            `json:"editor" yaml:"editor"`
	Terminal    Terminal            `json:"terminal" yaml:"terminal"`
	Shell       []string            `json:"shell" yaml:"shell"`
	KeepAlive   string              `json:"keep_alive" yaml:"keep_alive"`
	Timeout     string              `json:"timeout" yaml:"timeout"`
	Compact     Compact             `json:"compact" yaml:"compact"`
	Keys        map[string]string   `json:"keys" yaml:"keys"`
	Animations  bool                `json:"animations" yaml:"animations"`
}

type Compact struct {
	Mode      string  `json:"mode" yaml:"mode"`
	Threshold float64 `json:"threshold" yaml:"threshold"`
	KeepTurns int     `json:"keep_turns" yaml:"keep_turns"`
	Budget    int     `json:"budget" yaml:"budget"`
}

type Terminal struct {
	Background    string  `json:"background" yaml:"background"`
	Foreground    string  `json:"foreground" yaml:"foreground"`
	OverlayWidth  float64 `json:"overlay_width" yaml:"overlay_width"`
	OverlayHeight float64 `json:"overlay_height" yaml:"overlay_height"`
	SplitWidth    float64 `json:"split_width" yaml:"split_width"`
	Scrollback    int     `json:"scrollback" yaml:"scrollback"`
	Directory     string  `json:"directory" yaml:"directory"`
}

type Paths struct{ Config, Data, Cache string }

func Default() Config {
	return Config{Provider: "ollama", Providers: DefaultProviders(), Clipboard: []string{"wl-copy"}, Appearance: Appearance{Background: "theme", Mouse: true}, Execution: Execution{Mode: "agent", MaxSteps: 8, OutputContext: 8192}, Leader: "space", Bindings: DefaultBindings(), AutoStart: true, Version: 1, Host: "http://127.0.0.1:11434", Model: DefaultModel,
		Context: 8192, Predict: 2048, Threads: 6, Temperature: 0.3, Thinking: "auto", Theme: "phosphor",
		Editor: []string{"nvim"}, Shell: []string{"zsh", "-i"}, KeepAlive: "2m", Timeout: "10m",
		Terminal: Terminal{Background: "inherit", Foreground: "inherit", OverlayWidth: 0.7, OverlayHeight: 0.6, SplitWidth: 0.4, Scrollback: 5000},
		Compact:  Compact{Mode: "ask", Threshold: 0.9, KeepTurns: 2, Budget: 1536},
		Keys:     map[string]string{"output_share": "ctrl+g", "setting_help": "?", "pin_edit": "e", "pin_remove": "d", "thinking_toggle": "za", "send": "ctrl+s", "editor": "ctrl+e", "focus": "tab", "cancel": "ctrl+c", "quit": "ctrl+q", "terminal_overlay": "alt+e", "terminal_split": "alt+s", "terminal_unfocus": "ctrl+\\"}, Animations: true}
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
	var fields map[string]any
	if err := yaml.Unmarshal(b, &fields); err != nil {
		return c, fmt.Errorf("config: %w", err)
	}
	if _, ok := fields["bindings"]; ok {
		c.Bindings = nil
	}
	var d interface{ Decode(any) error }
	if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
		yd := yaml.NewDecoder(bytes.NewReader(b))
		yd.KnownFields(true)
		d = yd
	} else {
		jd := json.NewDecoder(bytes.NewReader(b))
		jd.DisallowUnknownFields()
		d = jd
	}
	if err := d.Decode(&c); err != nil {
		return c, fmt.Errorf("config: %w", err)
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return c, errors.New("config has trailing data")
	}
	if _, explicit := fields["model"]; !explicit && c.Providers[c.Provider].Model != "" {
		c.Model = c.Providers[c.Provider].Model
	}
	return c, c.Validate()
}

func Save(path string, c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	b, err := Encode(c, path)
	if err != nil {
		return err
	}
	return fsutil.Bytes(path, append(b, '\n'))
}

func (c Config) Validate() error {
	if err := c.ValidateProviders(); err != nil {
		return err
	}
	if !slices.Contains([]string{"theme", "inherit"}, c.Appearance.Background) {
		return errors.New("appearance.background must be theme or inherit")
	}
	if c.Keys["setting_help"] == "" || c.Keys["pin_edit"] == "" || c.Keys["pin_remove"] == "" || c.Keys["pin_edit"] == c.Keys["pin_remove"] || c.Keys["thinking_toggle"] == "" {
		return errors.New("pin and thinking bindings must be nonempty and pin bindings distinct")
	}
	if !slices.Contains([]string{"agent", "blocks"}, c.Execution.Mode) || c.Execution.MaxSteps < 1 || c.Execution.MaxSteps > 100 || c.Execution.OutputContext < 256 || c.Execution.OutputContext > 65536 {
		return errors.New("execution requires agent or blocks mode, 1-100 steps and 256-65536 output bytes")
	}
	if err := c.validateBindings(); err != nil {
		return err
	}
	if len(c.Clipboard) > 0 && strings.TrimSpace(c.Clipboard[0]) == "" {
		return errors.New("clipboard executable is empty")
	}
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
	if c.Context < 1 {
		return errors.New("context must be a positive integer")
	}
	if c.Predict < 1 {
		return errors.New("predict must be a positive integer")
	}
	if c.Threads < 0 || c.Threads > 512 {
		return errors.New("threads must be between 0 and 512")
	}
	if c.Temperature < 0 || c.Temperature > 2 {
		return errors.New("temperature must be between 0 and 2")
	}
	thinking := []string{"auto", "off", "on", "low", "medium", "high"}
	if p := c.Providers[c.Provider]; p.Kind != "ollama" {
		thinking = append(thinking, p.Thinking...)
	}
	if !slices.Contains(thinking, c.Thinking) {
		return errors.New("thinking must be a supported value from the selected provider")
	}
	if err := c.validateThemes(); err != nil {
		return err
	}
	for _, argv := range [][]string{c.Editor, c.Shell} {
		if len(argv) == 0 || strings.TrimSpace(argv[0]) == "" {
			return errors.New("editor and shell must be nonempty argument arrays")
		}
	}
	for _, argv := range [][]string{c.Editor, c.Shell, c.Clipboard} {
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
	if c.Compact.Budget < 256 {
		return errors.New("compact.budget must be at least 256")
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

func Encode(c Config, path string) ([]byte, error) {
	if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
		return yaml.Marshal(c)
	}
	return json.MarshalIndent(c, "", "  ")
}

func File(dir string) string { return filepath.Join(dir, "config.yaml") }

func LoadDirectory(dir string) (Config, error) {
	path := File(dir)
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		legacy := filepath.Join(dir, "config.json")
		if _, err := os.Stat(legacy); err == nil {
			c, err := Load(legacy)
			if err != nil {
				return c, err
			}
			return c, Save(path, c)
		}
	}
	return Load(path)
}
