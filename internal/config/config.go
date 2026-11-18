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
	"github.com/erdajt/wraith/internal/keymap"
	"gopkg.in/yaml.v3"
)

const DefaultModel = "hf.co/filvyb/Qwen3.5-9B-heretic-v2-GGUF:Q5_K_M"

type Execution struct {
	CaptureBytes   int    `json:"capture_bytes" yaml:"capture_bytes"`
	TailBytes      int    `json:"tail_bytes" yaml:"tail_bytes"`
	MaxJobs        int    `json:"max_jobs" yaml:"max_jobs"`
	MaxReads       int    `json:"max_reads" yaml:"max_reads"`
	CommandTimeout string `json:"command_timeout" yaml:"command_timeout"`
	AutoShare      bool   `json:"auto_share" yaml:"auto_share"`
	Mode           string `json:"mode" yaml:"mode"`
	MaxSteps       int    `json:"max_steps" yaml:"max_steps"`
	OutputContext  int    `json:"output_context" yaml:"output_context"`
}

type Environment struct {
	Scope string `json:"scope" yaml:"scope"`
}

type Appearance struct {
	Background string `json:"background" yaml:"background"`
	Mouse      bool   `json:"mouse" yaml:"mouse"`
	Thinking   bool   `json:"thinking" yaml:"thinking"`
}

type Config struct {
	Keymaps     []keymap.Binding    `json:"keymaps" yaml:"keymaps"`
	KeyTimeout  int                 `json:"key_timeout" yaml:"key_timeout"`
	Environment Environment         `json:"environment" yaml:"environment"`
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

// Default returns the application's default configuration.
func Default() Config {
	return Config{KeyTimeout: 750, Keymaps: []keymap.Binding{}, Environment: Environment{Scope: "directory"}, Provider: "ollama", Providers: DefaultProviders(), Clipboard: []string{"wl-copy"}, Appearance: Appearance{Background: "theme", Mouse: true}, Execution: Execution{CaptureBytes: 8 << 20, TailBytes: 64 << 10, MaxJobs: 4, MaxReads: 16, CommandTimeout: "10m", Mode: "agent", MaxSteps: 8, OutputContext: 8192}, Leader: "space", Bindings: DefaultBindings(), AutoStart: true, Version: 1, Host: "http://127.0.0.1:11434", Model: DefaultModel,
		Context: 8192, Predict: 2048, Threads: 6, Temperature: 0.3, Thinking: "auto", Theme: "phosphor",
		Editor: []string{"nvim"}, Shell: []string{"zsh", "-i"}, KeepAlive: "2m", Timeout: "10m",
		Terminal: Terminal{Background: "inherit", Foreground: "inherit", OverlayWidth: 0.7, OverlayHeight: 0.6, SplitWidth: 0.4, Scrollback: 5000},
		Compact:  Compact{Mode: "ask", Threshold: 0.9, KeepTurns: 2, Budget: 1536},
		Keys:     map[string]string{"job_cancel": "ctrl+x", "env_new": "n", "env_edit": "e", "env_remove": "d", "env_scope": "g", "output_share": "ctrl+g", "setting_help": "?", "pin_edit": "e", "pin_remove": "d", "thinking_toggle": "za", "send": "ctrl+s", "editor": "ctrl+e", "focus": "tab", "cancel": "ctrl+c", "quit": "ctrl+q", "terminal_overlay": "alt+e", "terminal_split": "alt+s", "terminal_unfocus": "ctrl+\\"}, Animations: true}
}

// Locations resolves configuration, data and cache locations from the environment.
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

// Load reads yaml or json over defaults and validates the resulting configuration.
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

// Save validates and atomically writes the configuration.
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

// Validate checks configuration values and dependent settings before activation.
func (c Config) Validate() error {
	if c.KeyTimeout < 50 || c.KeyTimeout > 5000 {
		return errors.New("key_timeout must be 50-5000 milliseconds")
	}
	if _, err := keymap.New(c.Keymaps); err != nil {
		return err
	}

	if !slices.Contains([]string{"conversation", "directory", "global"}, c.Environment.Scope) {
		return errors.New("environment.scope must be conversation, directory or global")
	}

	if err := c.ValidateProviders(); err != nil {
		return err
	}

	if !slices.Contains([]string{"theme", "inherit"}, c.Appearance.Background) {
		return errors.New("appearance.background must be theme or inherit")
	}

	if c.Keys["setting_help"] == "" || c.Keys["pin_edit"] == "" || c.Keys["pin_remove"] == "" || c.Keys["pin_edit"] == c.Keys["pin_remove"] || c.Keys["thinking_toggle"] == "" {
		return errors.New("pin and thinking bindings must be nonempty and pin bindings distinct")
	}

	seenEnv := map[string]bool{}
	for _, name := range []string{"env_new", "env_edit", "env_remove", "env_scope"} {
		binding := c.Keys[name]
		if binding == "" || seenEnv[binding] {
			return errors.New("environment bindings must be nonempty and distinct")
		}
		seenEnv[binding] = true
	}

	if !slices.Contains([]string{"agent", "blocks"}, c.Execution.Mode) || c.Execution.MaxSteps < 1 || c.Execution.MaxSteps > 100 || c.Execution.OutputContext < 256 || c.Execution.OutputContext > 65536 {
		return errors.New("execution requires agent or blocks mode, 1-100 steps and 256-65536 output bytes")
	}
	if c.Execution.CaptureBytes < 64<<10 || c.Execution.CaptureBytes > 256<<20 || c.Execution.TailBytes < 4<<10 || c.Execution.TailBytes > 1<<20 || c.Execution.TailBytes > c.Execution.CaptureBytes {
		return errors.New("capture bytes must be 64 kib–256 mib; tail bytes must be 4 kib–1 mib and within capture size")
	}
	duration, err := time.ParseDuration(c.Execution.CommandTimeout)
	if err != nil || duration <= 0 || c.Execution.MaxJobs < 1 || c.Execution.MaxJobs > 32 || c.Execution.MaxReads < 1 || c.Execution.MaxReads > 256 {
		return errors.New("execution requires a positive command timeout, 1-32 jobs and 1-256 reads")
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
	if !(c.Temperature >= 0 && c.Temperature <= 2) {
		return errors.New("temperature must be between 0 and 2")
	}

	thinking := []string{"auto", "off", "on", "low", "medium", "high"}
	if p := c.Providers[c.Provider]; p.Kind != "ollama" {
		thinking = append(thinking, p.Thinking...)
		if p.Kind == "codex-cli" && len(p.Thinking) == 0 && c.Thinking != "" && strings.TrimSpace(c.Thinking) == c.Thinking {
			thinking = append(thinking, c.Thinking)
		}
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

	if !(c.Terminal.OverlayWidth >= 0.3 && c.Terminal.OverlayWidth <= 0.95) ||
		!(c.Terminal.OverlayHeight >= 0.3 && c.Terminal.OverlayHeight <= 0.95) ||
		!(c.Terminal.SplitWidth >= 0.25 && c.Terminal.SplitWidth <= 0.7) {
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
	if !(c.Compact.Threshold >= 0.5 && c.Compact.Threshold <= 0.95) {
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

// validColor reports whether a string is a supported hexadecimal color.
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

// Encode encodes configuration in the format selected by path.
func Encode(c Config, path string) ([]byte, error) {
	if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
		encoded, err := json.Marshal(c)
		if err != nil {
			return nil, err
		}
		var node yaml.Node
		if err := yaml.Unmarshal(encoded, &node); err != nil {
			return nil, err
		}
		pending := []*yaml.Node{&node}
		for len(pending) > 0 {
			current := pending[len(pending)-1]
			pending = pending[:len(pending)-1]
			current.Style = 0
			if current.Tag == "!!str" && strings.ContainsAny(current.Value, "\r\n") {
				current.Style = yaml.DoubleQuotedStyle
			}
			pending = append(pending, current.Content...)
		}
		return yaml.Marshal(&node)
	}
	return json.MarshalIndent(c, "", "  ")
}

// File returns the configuration filename within dir.
func File(dir string) string { return filepath.Join(dir, "config.yaml") }

// LoadDirectory loads configuration from dir, including supported legacy files.
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
