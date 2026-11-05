package ui

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/editor"
	"github.com/erdajt/wraith/internal/fsutil"
)

func (a *App) memoryCommand(name, arg string) tea.Cmd {
	switch name {
	case "help", "h", "?":
		a.popup = &menu{Kind: "help", Title: "wraith · command reference", Items: helpItems()}
	case "instructions", "memory":
		return a.edit("instructions", "", "", -1)
	case "remember":
		if arg == "" {
			a.notice = "usage: /remember an instruction for every chat"
			return nil
		}
		next := strings.TrimSpace(a.Instructions) + "\n- " + arg + "\n"
		if len(next) > 64<<10 {
			a.notice = "persistent instructions exceed 64 kib"
			return nil
		}
		if err := fsutil.Bytes(filepath.Join(a.Paths.Config, "instructions.md"), []byte(next)); err != nil {
			a.notice = err.Error()
		} else {
			a.Instructions = next
			a.notice = "instruction saved for every chat"
		}
	case "pin":
		if arg == "" {
			a.notice = "usage: /pin an exact fact for this chat"
			return nil
		}
		a.Session.Pins = append(a.Session.Pins, arg)
		a.persist("fact pinned verbatim")
	case "unpin":
		n, err := strconv.Atoi(arg)
		if err != nil || n < 1 || n > len(a.Session.Pins) {
			a.notice = "usage: /unpin number from /pins"
			return nil
		}
		a.Session.Pins = append(a.Session.Pins[:n-1], a.Session.Pins[n:]...)
		a.persist("pin removed")
	case "pins":
		items := []item{}
		for i, p := range a.Session.Pins {
			items = append(items, item{fmt.Sprintf("%d · %s", i+1, p), strconv.Itoa(i), ""})
		}
		a.popup = &menu{Kind: "pins", Title: "exact pins · " + a.Config.Keys["pin_edit"] + " edit · " + a.Config.Keys["pin_remove"] + " unpin", Items: items}
	case "blocks":
		items := []item{}
		for mi, m := range a.Session.Messages {
			if m.Role != "assistant" {
				continue
			}
			for bi, b := range editor.Blocks(m.Content) {
				first, _, _ := strings.Cut(strings.TrimSpace(b.Text), "\n")
				items = append(items, item{fmt.Sprintf("%d · %s · %s", len(items)+1, b.Language, first), fmt.Sprintf("%d:%d", mi, bi), fmt.Sprintf("message %d", mi+1)})
			}
		}
		a.popup = &menu{Kind: "blocks", Title: "code blocks · enter chooses edit or run", Items: items}
		if len(items) > 0 {
			a.popup.Index = len(items) - 1
		}
	case "edit":
		return a.edit("prompt", "", "", -1)
	case "edit-response":
		return a.editResponse()
	}
	return nil
}
