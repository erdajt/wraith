package ui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/contextual"
	"github.com/erdajt/wraith/internal/editor"
	"github.com/erdajt/wraith/internal/fsutil"
	"github.com/erdajt/wraith/internal/keymap"
	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/plugin"
	"github.com/erdajt/wraith/internal/provider"
	"github.com/erdajt/wraith/internal/runner"
	"github.com/erdajt/wraith/internal/session"
	"github.com/erdajt/wraith/internal/store"
	"github.com/erdajt/wraith/internal/terminal"
	"github.com/erdajt/wraith/internal/variables"
	"github.com/erdajt/wraith/internal/vim"
)

type item struct{ Label, Value, Detail string }
type menu struct {
	Reader               *reader
	Record               runner.Record
	Input                *vim.Buffer
	All                  []item
	Query, Prefix, Count string
	Searching            bool
	Kind, Title, Hints   string
	Items                []item
	Index                int
}
type pane struct {
	SharedText              string
	Reader                  *reader
	Escapes                 int
	EscapedAt               time.Time
	Started                 time.Time
	Restarts                int
	PTY                     *terminal.PTY
	Screen                  *terminal.Screen
	Visible                 bool
	Pending, Path, Language string
	PasteID                 string
	PasteCancelled          bool
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
	Serial uint64
	Models []ollama.Model
	Err    error
}
type streamMsg struct {
	Submission *chatSubmission
	Rejected   session.Message
	Serial     uint64
	Retry      int
	Problem    string
	Chunk      ollama.Response
	Final      bool
	Err        error
}

type chatSubmission struct {
	Before session.Session
	Draft  string
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
	configEditBase     *config.Config
	pluginLoadCancel   context.CancelFunc
	plugins            *plugin.Runtime
	pluginManifest     plugin.Manifest
	pluginDiagnostics  []string
	pluginSerial       uint64
	bufferEventPending bool
	bufferEventRuntime *plugin.Runtime
	pluginDepth        int
	pluginInvocation   string
	policySerial       uint64
	policyCancel       context.CancelFunc
	pluginQueries      map[string]*workspaceQueryState
	workspaceScans     *workspaceScanStore
	workspaceEvents    workspaceEventState
	surfaceReplay      bool
	surfaceKeymaps     map[string]surfaceKeymapCache
	surfaceBuffers     map[string]plugin.SurfaceBuffer
	surfaceRegions     map[string]plugin.SurfaceRegion
	surfaceWindows     map[string]uint64
	surfaceRuntime     *plugin.Runtime
	pluginDialog       *pluginDialog
	nativeProjection   *nativeProjection

	editorFile        *editor.File
	editorSucceeded   bool
	editorDeferred    []tea.Msg
	keyPending        []tea.KeyMsg
	keyPendingDefault bool
	keyPendingScope   string
	keySerial         uint64
	actions           map[string]action
	keymaps           *keymap.Registry
	*conversationView
	windows                       map[uint64]*conversationView
	windowTree                    *windowNode
	windowID, nextWindowID        uint64
	Variables                     variables.Snapshot
	variableInventory, envPending string
	envScope                      variables.Scope
	envEdit                       struct {
		Store    variables.Store
		Scope    variables.Scope
		Revision string
	}

	streamStopped bool
	streamSerial  uint64
	jobs          *runner.Manager

	pendingRuns                       map[string]runner.Record
	generation                        string
	agentStopped                      bool
	editing                           bool
	receiptSession, receiptGeneration string

	pendingTool *contextual.ToolRequest
	agentReads  int
	lastPoll    map[string]time.Time
	pollNext    map[string]int64

	inputHistory                   []string
	inputIndex, inputDraftPosition int
	inputDraft                     string
	Directory                      string
	forkIndex                      int
	helpReturn                     *menu
	thinkingExpanded               map[string]bool
	agentSteps                     int
	pendingAgent                   bool
	selectedBlock                  editor.Block
	proposal                       *proposal
	executing                      bool
	activeRun                      runner.Record
	workers                        sync.WaitGroup
	Config                         config.Config
	Paths                          config.Paths
	Store                          store.Store
	Session                        session.Session
	Client                         *provider.Client
	Info                           ollama.Info
	Instructions                   string
	Prompt                         *vim.Buffer
	width, height                  int
	focus                          string
	popup                          *menu
	command                        string
	completionQuery                string
	completionIndex                int
	commandMode, searchMode        bool
	panes                          [2]*pane
	busy, compacting               bool
	partial                        session.Message
	cancel                         context.CancelFunc
	events                         chan tea.Msg
	done                           chan struct{}
	once                           sync.Once
	notice                         string
	frame                          int
	phase                          string
	loadCancel                     context.CancelFunc
	loadSerial                     uint64
	catalogSerial                  uint64
	loading                        bool
	connected                      bool
	demo                           bool
	showThinking                   bool
	pendingPrompt                  string
	resumePrompt                   *string
}

// New initializes the workspace, loads private context and recovers command history outside demo mode.
func New(c config.Config, p config.Paths, s session.Session, demo bool, extensions ...*plugin.Runtime) (*App, error) {
	instructions, err := fsutil.ReadLimit(filepath.Join(p.Config, "instructions.md"), 64<<10)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if demo {
		instructions = nil
	}

	c.Model = s.Model
	if s.Provider != "" {
		c.Provider = s.Provider
	}
	if _, ok := c.Providers[c.Provider]; !ok {
		return nil, errors.New("session provider is not configured")
	}

	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if real, e := filepath.EvalSymlinks(dir); e == nil {
		dir = real
	}

	a := &App{inputIndex: -1, Directory: dir, Config: c, Paths: p, Store: store.Store{Root: p.Data}, Session: s, Client: provider.New(c), Instructions: string(instructions), Prompt: vim.New(), width: 100, height: 30, conversationView: &conversationView{chatCursor: -1, follow: true}, focus: "prompt", events: make(chan tea.Msg, 128), done: make(chan struct{}), demo: demo, notice: "/help commands · " + c.Keys["send"] + " send · " + c.Keys["editor"] + " editor"}

	a.resetPrompt(s.Draft)
	a.windowID, a.nextWindowID = 1, 1
	a.windows = map[uint64]*conversationView{1: a.conversationView}
	a.windowTree = &windowNode{id: 1}
	a.actions = a.builtinActions()
	if len(extensions) > 0 && extensions[0] != nil {
		a.plugins = extensions[0]
		a.pluginManifest = a.plugins.Manifest()
		a.actions = a.pluginActions(a.pluginManifest)
	}
	a.Client = a.providerClient(c)
	if a.keymaps, err = a.buildKeymaps(c); err != nil {
		return nil, err
	}

	if !demo {
		if err := a.refreshVariables(); err != nil {
			return nil, err
		}

		entries, e := a.Store.Inputs(a.Directory)
		if e != nil {
			a.notice = "input history: " + e.Error()
		} else {
			a.inputHistory = entries
		}
	}

	a.agentStopped = true
	a.pendingRuns = map[string]runner.Record{}
	a.lastPoll = map[string]time.Time{}
	a.pollNext = map[string]int64{}
	a.jobs = runner.NewManager(a.history(), c.Execution.MaxJobs)
	if !demo {
		if recovered, e := a.history().Recover(); e != nil {
			a.notice = "command recovery: " + e.Error()
		} else if len(recovered) > 0 {
			a.notice = "recovered interrupted commands · /history"
		}
	}
	if !demo {
		a.reconcileJobs()
	}

	a.showThinking = c.Appearance.Thinking
	a.thinkingExpanded = map[string]bool{}
	if demo {
		a.connected = true
		a.notice = "demo workspace · no model requests"
		a.Session.Title = "tls assessment"
		a.Session.Messages = []session.Message{{Role: "user", Content: "Review the TLS configuration on my isolated lab service. Preserve the hostname and verification steps."}, {Role: "assistant", Content: "Start by inspecting the certificate and negotiated protocol. This checks the service without changing its configuration.\n\n```sh\nopenssl s_client -connect lab.internal:443 \\\n  -servername lab.internal -brief </dev/null\n```\n\nRecord the protocol, cipher suite, issuer, and expiration. A successful handshake alone does not establish that every supported protocol is acceptable.\n\nNext: compare the observed settings with your expected TLS policy."}}
	}
	if len(a.pluginManifest.Diagnostics) > 0 {
		a.notice += " · lua: " + strings.ReplaceAll(clean(strings.Join(a.pluginManifest.Diagnostics, " · ")), "\n", " ")
	}
	surfaces, err := preparePluginSurfaces(a.plugins, 1)
	if err != nil {
		a.Close()
		return nil, err
	}
	a.installPluginSurfaces(surfaces)
	return a, nil
}

// Init starts ui ticks and the initial model probe.
func (a *App) Init() tea.Cmd {
	if a.demo {
		return a.ticker()
	}
	return tea.Batch(a.ticker(), a.probe(), a.callPlugin("", "ready", ""), a.watchPluginNotices(a.plugins), a.startPluginResources(), a.watchPluginInvocations(a.plugins))
}

// ticker schedules the next animation and job-update tick.
func (a *App) ticker() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg { return tick(t) })
}

// Close cancels active work and closes owned shell panes once.
func (a *App) Close() {
	a.once.Do(func() {
		a.reconcileWorkspaceQueries(true)
		if a.pluginLoadCancel != nil {
			a.pluginLoadCancel()
		}
		if a.plugins != nil {
			a.plugins.Close()
		}
		if a.loadCancel != nil {
			a.loadCancel()
		}
		if a.cancel != nil {
			a.cancel()
		}
		if a.policyCancel != nil {
			a.policyCancel()
		}
		close(a.done)

		if a.jobs != nil {
			a.jobs.Close()
		}

		a.workers.Wait()

		for _, p := range a.panes {
			if p != nil && p.PTY != nil {
				p.PTY.Close()
			}
		}
	})
}
