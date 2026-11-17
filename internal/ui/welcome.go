package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

var wraithWordmark = []string{
	"█   █  █▀▀▄  ▄▀▀▄  ▀█▀  ▀▀█▀▀  █   █",
	"█ ▄ █  █▄▄▀  █▄▄█   █     █    █▄▄▄█",
	"█▀ ▀█  █  █  █  █  ▄█▄    █    █   █",
}

func wraithEmblem(height, frame int, p palette) []string {
	art := []string{
		"         ▄██▄         ",
		"      ▄████████▄      ",
		"    ▄███▀    ▀███▄    ",
		"   ███▀        ▀███   ",
		"   ██▌  ▀▄  ▄▀  ▐██   ",
		"   ██▌          ▐██   ",
		"   ▐██▄        ▄██▌   ",
		"    ▀███▄    ▄███▀    ",
		"     ██▀██▄▄██▀██     ",
		"     ▀   ▀██▀   ▀     ",
	}
	if height < 18 {
		art = []string{
			"       ▄██▄       ",
			"    ▄██▀  ▀██▄    ",
			"   ██ ▀▄  ▄▀ ██   ",
			"   ▐█▄      ▄█▌   ",
			"    ▀██▄▄▄▄██▀    ",
			"     ▀ ▀██▀ ▀     ",
		}
	}
	phase := frame / 3 % 12
	rows := make([]string, len(art))
	for i, line := range art {
		shade := p.Second
		if i == phase || i == phase-1 {
			shade = p.Accent
		}
		if before, after, ok := strings.Cut(line, "▀▄  ▄▀"); ok {
			eyes := p.Accent
			if phase == 5 || phase == 6 {
				eyes = p.FG
			}
			rows[i] = ink(shade, before) + bold(eyes, "▀▄  ▄▀") + ink(shade, after)
		} else {
			rows[i] = ink(shade, line)
		}
		rows[i] = fit(rows[i], ansi.StringWidth(art[0]))
	}
	return rows
}

func (a *App) welcome(width, height int, p palette) []string {
	frame := a.frame
	if !a.Config.Animations {
		frame = 0
	}
	var rows []string
	if height >= 10 {
		rows = append(rows, wraithEmblem(height, frame, p)...)
		rows = append(rows, "")
	}
	if width >= 48 && height >= 18 {
		for _, line := range wraithWordmark {
			rows = append(rows, bold(p.Accent, line))
		}
	} else {
		rows = append(rows, bold(p.Accent, "W R A I T H"))
	}
	rows = append(rows, "", ansi.Truncate(ink(p.Muted, "write a prompt · ")+keyHints("/models choose a model", p), max(1, width-4), ""))
	return rows
}
