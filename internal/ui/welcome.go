package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

var wraithWordmark = []string{
	` _       ______  ___    __________________`,
	`| |     / / __ \/   |  /  _/_  __/ / / /`,
	`| | /| / / /_/ / /| |  / /  / / / /_/ /`,
	`| |/ |/ / _, _/ ___ |_/ /  / / / __  /`,
	`|__/|__/_/ |_/_/  |_/___/ /_/ /_/ /_/`,
}

func wraithEmblem(height, frame int, p palette) []string {
	art := []string{
		`          /\`,
		`        _/  \_`,
		`       / .--. \`,
		`      / /    \ \`,
		`     | | \  / | |`,
		`     |  \    /  |`,
		`      \_/'--'\_/`,
		`        / /\ \`,
	}
	if height >= 20 {
		art = []string{
			`           /\`,
			`          /  \`,
			`        _/    \_`,
			`       /  .--.  \`,
			`      /  /    \  \`,
			`     |  | \  / |  |`,
			`     |  |      |  |`,
			`     |   \    /   |`,
			`      \   '--'   /`,
			`       \_/|  |\_/`,
			`         / /\ \`,
			`        /_/  \_\`,
		}
	} else if height < 14 {
		art = []string{
			`         /\`,
			`       /    \`,
			`      | \  / |`,
			`      |      |`,
			`       \_/\_/`,
		}
	}
	phase := frame / 3 % 8
	rows := make([]string, len(art))
	for i, line := range art {
		line = fit(line, 25)
		if i >= len(art)-3 {
			left, right := byte(' '), byte(' ')
			if i == len(art)-1-phase%3 {
				left = []byte{'.', ':', '\'', ' '}[phase%4]
				right = []byte{' ', '.', ':', '\''}[phase%4]
			}
			line = string(left) + line[1:24] + string(right)
		}
		if before, after, ok := strings.Cut(line, `\  /`); ok {
			eyes := p.Accent
			if phase == 3 || phase == 4 {
				eyes = p.FG
			}
			rows[i] = ink(p.Second, before) + bold(eyes, `\  /`) + ink(p.Second, after)
		} else {
			rows[i] = ink(p.Second, line)
		}
	}
	return rows
}

func (a *App) welcome(width, height int, p palette) []string {
	frame := a.frame
	if !a.Config.Animations {
		frame = 0
	}
	var rows []string
	if height >= 9 {
		rows = append(rows, wraithEmblem(height, frame, p)...)
		rows = append(rows, "")
	}
	if width >= 48 && height >= 18 {
		for _, line := range wraithWordmark {
			rows = append(rows, bold(p.Accent, fit(line, 41)))
		}
	} else {
		rows = append(rows, bold(p.Accent, "W R A I T H"))
	}
	rows = append(rows, "", ink(p.Muted, ansi.Truncate("write a prompt · /models to choose a model", max(1, width-4), "")))
	return rows
}
