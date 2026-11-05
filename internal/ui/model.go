package ui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/editor"
	"github.com/erdajt/wraith/internal/fsutil"
	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/runner"
	"github.com/erdajt/wraith/internal/session"
	"github.com/erdajt/wraith/internal/store"
	"github.com/erdajt/wraith/internal/terminal"
	"github.com/erdajt/wraith/internal/vim"
)

type item struct{ Label, Value, Detail string }
type menu struct {
	All                  []item
	Query, Prefix, Count string
	Searching            bool
	Kind, Title          string
	Items                []item
	Index                int
}
type pane struct {
	PTY                     *terminal.PTY
	Screen                  *terminal.Screen
	Visible                 bool
	Pending, Path, Language string
	Err                     string
}
type tick time.Time
type infoMsg struct {
	Serial uint64
	Model  string
	Info   ollama.Info
	Err    error
}
type modelsMsg struct {
	Models []ollama.Model
	Err    error
}
type streamMsg struct {
	Chunk ollama.Response
	Final bool
	Err   error
}
type compactMsg struct {
	Session session.Session
	Archive string
	Err     error
}
type progressMsg string
type terminalMsg struct {
	Index int
	PTY   *terminal.PTY
	Event terminal.Event
	Open  bool
}

type App struct {
	chatCursor, chatColumn  int
	visualMode              string
	visualRow, visualColumn int
	helpReturn              *menu
	thinkingExpanded        map[string]bool
	agentSteps              int
	pendingAgent            bool
	selectedBlock           editor.Block
	proposal                *proposal
	executing, runContinue  bool
	activeRun               runner.Record
	runPreview              string
	workers                 sync.WaitGroup
	Config                  config.Config
	Paths                   config.Paths
	Store                   store.Store
	Session                 session.Session
	Client                  *ollama.Client
	Info                    ollama.Info
	Instructions            string
	Prompt                  *vim.Buffer
	width, height           int
	focus                   string
	offset, selected        int
	follow                  bool
	popup                   *menu
	command, search         string
	chatCount               string
	chatG                   bool
	completionQuery         string
	completionIndex         int
	commandMode, searchMode bool
	panes                   [2]*pane
	busy, compacting        bool
	partial                 session.Message
	cancel                  context.CancelFunc
	events                  chan tea.Msg
	done                    chan struct{}
	once                    sync.Once
	notice                  string
	frame                   int
	phase                   string
	loadCancel              context.CancelFunc
	loadSerial              uint64
	loading                 bool
	connected               bool
	demo                    bool
	showThinking            bool
	pendingPrompt           string
}

func New(c config.Config, p config.Paths, s session.Session, demo bool) (*App, error) {
	instructions, err := fsutil.ReadLimit(filepath.Join(p.Config, "instructions.md"), 64<<10)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if demo {
		instructions = nil
	}
	c.Model = s.Model
	a := &App{Config: c, Paths: p, Store: store.Store{Root: p.Data}, Session: s, Client: ollama.New(c.Host), Instructions: string(instructions), Prompt: vim.New(), width: 100, height: 30, chatCursor: -1, focus: "prompt", follow: true, events: make(chan tea.Msg, 128), done: make(chan struct{}), demo: demo, notice: "/help commands · ctrl+s send · ctrl+e editor"}
	a.showThinking = c.Appearance.Thinking
	a.thinkingExpanded = map[string]bool{}
	if demo {
		a.connected = true
		a.notice = "demo workspace · no model requests"
		a.Session.Title = "tls assessment"
		a.Session.Messages = []session.Message{{Role: "user", Content: "Review the TLS configuration on my isolated lab service. Preserve the hostname and verification steps."}, {Role: "assistant", Content: "Start by inspecting the certificate and negotiated protocol. This checks the service without changing its configuration.\n\n```sh\nopenssl s_client -connect lab.internal:443 \\\n  -servername lab.internal -brief </dev/null\n```\n\nRecord the protocol, cipher suite, issuer, and expiration. A successful handshake alone does not establish that every supported protocol is acceptable.\n\nNext: compare the observed settings with your expected TLS policy."}}
	}
	return a, nil
}

func (a *App) Init() tea.Cmd {
	if a.demo {
		return a.ticker()
	}
	return tea.Batch(a.ticker(), a.probe())
}
func (a *App) ticker() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg { return tick(t) })
}
func (a *App) Close() {
	a.once.Do(func() {
		if a.loadCancel != nil {
			a.loadCancel()
		}
		if a.cancel != nil {
			a.cancel()
		}
		close(a.done)
		a.workers.Wait()
		for _, p := range a.panes {
			if p != nil && p.PTY != nil {
				p.PTY.Close()
			}
		}
	})
}
