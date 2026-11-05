package ui

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/fsutil"
	"github.com/erdajt/wraith/internal/session"
	"github.com/erdajt/wraith/internal/vim"
)

func (a *App) sessionCommand(name, arg string) tea.Cmd {
	switch name {
	case "new":
		s := session.New(a.Config.Model)
		if err := a.Store.Save(s); err != nil {
			a.notice = err.Error()
			return nil
		}
		a.Session = s
		a.selected = 0
		a.offset = 0
		a.follow = true
		a.Prompt = vim.New()
		a.pendingPrompt = ""
		a.notice = "new chat · persistent instructions retained"
		return a.probe()
	case "chats", "sessions":
		list, err := a.Store.List()
		if err != nil {
			a.notice = err.Error()
			return nil
		}
		items := []item{}
		for _, s := range list {
			items = append(items, item{s.Title, s.ID, s.Updated.Local().Format("jan 02 15:04") + " · " + s.ID[:8]})
		}
		a.popup = &menu{Kind: "sessions", Title: "saved chats", Items: items}
	case "resume":
		return a.resume(arg)
	case "archives":
		paths, err := filepath.Glob(filepath.Join(a.Paths.Data, "archives", a.Session.ID+"-*.json.gz"))
		if err != nil {
			a.notice = err.Error()
			return nil
		}
		items := []item{}
		for _, p := range paths {
			info, err := os.Stat(p)
			if err != nil {
				continue
			}
			items = append(items, item{filepath.Base(p), filepath.Base(p), info.ModTime().Local().Format("jan 02 15:04:05")})
		}
		a.popup = &menu{Kind: "archives", Title: "archives · enter forks the original into a new chat", Items: items}
	case "restore":
		return a.restore(arg)
	case "export":
		if arg == "" {
			arg = filepath.Join(a.Paths.Data, "exports", a.Session.ID+"-"+time.Now().Format("20060102-150405")+".md")
		}
		if strings.HasPrefix(arg, "~/") {
			home, _ := os.UserHomeDir()
			arg = filepath.Join(home, arg[2:])
		}
		b := Export(a.Session)
		if err := fsutil.Bytes(arg, []byte(b)); err != nil {
			a.notice = err.Error()
		} else {
			a.notice = "exported " + arg
		}
	case "retry":
		if n := len(a.Session.Messages); n > 0 && a.Session.Messages[n-1].Interrupted {
			a.Session.Messages = a.Session.Messages[:n-1]
		}
		if n := len(a.Session.Messages); n == 0 || a.Session.Messages[n-1].Role != "user" {
			a.notice = "no interrupted request to retry"
			return nil
		}
		return a.startChat()
	case "terminal-restart":
		n, err := strconv.Atoi(arg)
		if err != nil || n < 1 || n > 2 {
			a.notice = "usage: /terminal-restart 1 or 2"
			return nil
		}
		if p := a.panes[n-1]; p != nil && p.PTY != nil {
			p.PTY.Close()
		}
		a.panes[n-1] = nil
		return a.toggleTerminal(n - 1)
	case "quit", "q":
		a.Close()
		return tea.Quit
	}
	return nil
}
