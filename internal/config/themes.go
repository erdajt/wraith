package config

import "fmt"

type Palette struct {
	BG     string `json:"bg" yaml:"bg"`
	FG     string `json:"fg" yaml:"fg"`
	Muted  string `json:"muted" yaml:"muted"`
	Accent string `json:"accent" yaml:"accent"`
	Second string `json:"second" yaml:"second"`
	Border string `json:"border" yaml:"border"`
	Warn   string `json:"warn" yaml:"warn"`
}

func BuiltinThemes() map[string]Palette {
	return map[string]Palette{
		"rose-pine":        {"#191724", "#e0def4", "#908caa", "#c4a7e7", "#ebbcba", "#403d52", "#f6c177"},
		"dracula":          {"#282a36", "#f8f8f2", "#6272a4", "#bd93f9", "#ff79c6", "#44475a", "#ffb86c"},
		"everforest":       {"#2d353b", "#d3c6aa", "#859289", "#a7c080", "#7fbbb3", "#475258", "#dbbc7f"},
		"nord":             {"#2e3440", "#eceff4", "#81a1c1", "#88c0d0", "#b48ead", "#4c566a", "#ebcb8b"},
		"catppuccin-mocha": {"#1e1e2e", "#cdd6f4", "#a6adc8", "#cba6f7", "#89b4fa", "#45475a", "#fab387"},
		"phosphor":         {"#101419", "#d8e4df", "#788b87", "#73f5bd", "#bb9aff", "#354743", "#ffb86b"},
		"tokyo":            {"#161822", "#c0caf5", "#737ca6", "#7aa2f7", "#bb9af7", "#353b5a", "#e0af68"},
		"ember":            {"#1b1515", "#f0ded3", "#aa8b82", "#ffb454", "#f47c91", "#55403a", "#ff6969"},
		"ice":              {"#10191f", "#d8eff5", "#7195a3", "#76e4f7", "#adbaff", "#304b58", "#ffb8a3"},
		"mono":             {"#151515", "#e0e0e0", "#888888", "#ffffff", "#bcbcbc", "#444444", "#eeeeee"},
	}
}

func (c Config) Palette() Palette {
	if p, ok := c.Themes[c.Theme]; ok {
		return p
	}
	return BuiltinThemes()[c.Theme]
}

func (c Config) validateThemes() error {
	for name, p := range c.Themes {
		if name == "" {
			return fmt.Errorf("theme name is empty")
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
