package config

import "fmt"

type Palette struct {
	HintQuit    string `json:"hint_quit,omitempty" yaml:"hint_quit,omitempty"`
	HintRefresh string `json:"hint_refresh,omitempty" yaml:"hint_refresh,omitempty"`
	HintCancel  string `json:"hint_cancel,omitempty" yaml:"hint_cancel,omitempty"`
	Variable    string `json:"variable,omitempty" yaml:"variable,omitempty"`
	BG          string `json:"bg" yaml:"bg"`
	FG          string `json:"fg" yaml:"fg"`
	Muted       string `json:"muted" yaml:"muted"`
	Accent      string `json:"accent" yaml:"accent"`
	Second      string `json:"second" yaml:"second"`
	Border      string `json:"border" yaml:"border"`
	Warn        string `json:"warn" yaml:"warn"`
}

func BuiltinThemes() map[string]Palette {
	return map[string]Palette{
		"rose-pine": {
			Variable: "#73f5bd", BG: "#191724", FG: "#e0def4",
			Muted: "#908caa", Accent: "#c4a7e7", Second: "#ebbcba",
			Border: "#403d52", Warn: "#f6c177",
			HintQuit: "#f38ba8", HintRefresh: "#89b4fa", HintCancel: "#f9e2af",
		},
		"dracula": {
			Variable: "#73f5bd", BG: "#282a36", FG: "#f8f8f2",
			Muted: "#6272a4", Accent: "#bd93f9", Second: "#ff79c6",
			Border: "#44475a", Warn: "#ffb86c",
			HintQuit: "#f38ba8", HintRefresh: "#89b4fa", HintCancel: "#f9e2af",
		},
		"everforest": {
			Variable: "#73f5bd", BG: "#2d353b", FG: "#d3c6aa",
			Muted: "#859289", Accent: "#a7c080", Second: "#7fbbb3",
			Border: "#475258", Warn: "#dbbc7f",
			HintQuit: "#f38ba8", HintRefresh: "#89b4fa", HintCancel: "#f9e2af",
		},
		"nord": {
			Variable: "#73f5bd", BG: "#2e3440", FG: "#eceff4",
			Muted: "#81a1c1", Accent: "#88c0d0", Second: "#b48ead",
			Border: "#4c566a", Warn: "#ebcb8b",
			HintQuit: "#f38ba8", HintRefresh: "#89b4fa", HintCancel: "#f9e2af",
		},
		"catppuccin-mocha": {
			Variable: "#73f5bd", BG: "#1e1e2e", FG: "#cdd6f4",
			Muted: "#a6adc8", Accent: "#cba6f7", Second: "#89b4fa",
			Border: "#45475a", Warn: "#fab387",
			HintQuit: "#f38ba8", HintRefresh: "#89b4fa", HintCancel: "#f9e2af",
		},
		"phosphor": {
			Variable: "#73f5bd", BG: "#101419", FG: "#d8e4df",
			Muted: "#788b87", Accent: "#73f5bd", Second: "#bb9aff",
			Border: "#354743", Warn: "#ffb86b",
			HintQuit: "#f38ba8", HintRefresh: "#89b4fa", HintCancel: "#f9e2af",
		},
		"tokyo": {
			Variable: "#73f5bd", BG: "#161822", FG: "#c0caf5",
			Muted: "#737ca6", Accent: "#7aa2f7", Second: "#bb9af7",
			Border: "#353b5a", Warn: "#e0af68",
			HintQuit: "#f38ba8", HintRefresh: "#89b4fa", HintCancel: "#f9e2af",
		},
		"ember": {
			Variable: "#73f5bd", BG: "#1b1515", FG: "#f0ded3",
			Muted: "#aa8b82", Accent: "#ffb454", Second: "#f47c91",
			Border: "#55403a", Warn: "#ff6969",
			HintQuit: "#f38ba8", HintRefresh: "#89b4fa", HintCancel: "#f9e2af",
		},
		"ice": {
			Variable: "#73f5bd", BG: "#10191f", FG: "#d8eff5",
			Muted: "#7195a3", Accent: "#76e4f7", Second: "#adbaff",
			Border: "#304b58", Warn: "#ffb8a3",
			HintQuit: "#f38ba8", HintRefresh: "#89b4fa", HintCancel: "#f9e2af",
		},
		"mono": {
			Variable: "#73f5bd", BG: "#151515", FG: "#e0e0e0",
			Muted: "#888888", Accent: "#ffffff", Second: "#bcbcbc",
			Border: "#444444", Warn: "#eeeeee",
			HintQuit: "#f38ba8", HintRefresh: "#89b4fa", HintCancel: "#f9e2af",
		},
	}
}

func (c Config) Palette() Palette {
	p, ok := c.Themes[c.Theme]
	if !ok {
		p = BuiltinThemes()[c.Theme]
	}
	if p.Variable == "" {
		p.Variable = "#73f5bd"
	}
	if p.HintQuit == "" {
		p.HintQuit = "#f38ba8"
	}
	if p.HintRefresh == "" {
		p.HintRefresh = "#89b4fa"
	}
	if p.HintCancel == "" {
		p.HintCancel = "#f9e2af"
	}
	if c.Appearance.Background == "inherit" {
		p.BG = ""
	}
	return p
}

func (c Config) validateThemes() error {
	for name, p := range c.Themes {
		if name == "" {
			return fmt.Errorf("theme name is empty")
		}
		for key, value := range map[string]string{"variable": p.Variable, "hint_quit": p.HintQuit, "hint_refresh": p.HintRefresh, "hint_cancel": p.HintCancel} {
			if value != "" && !validColor(value) {
				return fmt.Errorf("theme %s %s color requires #rrggbb", name, key)
			}
		}
		for _, v := range []string{p.BG, p.FG, p.Muted, p.Accent, p.Second, p.Border, p.Warn} {
			if !validColor(v) {
				return fmt.Errorf("theme %s requires seven #rrggbb colors", name)
			}
		}
	}
	if _, ok := BuiltinThemes()[c.Theme]; !ok {
		if _, ok := c.Themes[c.Theme]; !ok {
			return fmt.Errorf("unknown theme: %s", c.Theme)
		}
	}
	return nil
}
