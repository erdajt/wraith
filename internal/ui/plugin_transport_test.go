package ui

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/contextual"
	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/provider"
	"github.com/erdajt/wraith/internal/session"
	"github.com/erdajt/wraith/internal/variables"
)

type transportDone struct{ err error }
type transportProgram struct {
	app  *App
	work func() error
	err  error
}

func (p *transportProgram) Init() tea.Cmd {
	return tea.Batch(p.app.watchPluginInvocations(p.app.plugins), p.app.startPluginResources(), func() tea.Msg { return transportDone{p.work()} })
}
func (p *transportProgram) View() string { return "" }
func (p *transportProgram) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if done, ok := msg.(transportDone); ok {
		p.err = done.err
		return p, tea.Quit
	}
	_, cmd := p.app.Update(msg)
	return p, cmd
}

// runTransport exercises worker calls against the actual actor and application message loop.
func runTransport(t *testing.T, a *App, work func() error) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	model := &transportProgram{app: a, work: work}
	program := tea.NewProgram(model, tea.WithContext(ctx), tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer(), tea.WithoutSignalHandler())
	if _, err := program.Run(); err != nil {
		t.Fatalf("transport did not finish: %v; %s", err, a.notice)
	}
	return model.err
}

func TestLuaProviderDiscoveryStreamingAndNativeActions(t *testing.T) {
	a := pluginFixture(t, `wraith.providers.register("custom",{
 models=function(ctx) wraith.invocation.finish(ctx.invocation.id,{{name="local",size=123}}) end,
 show=function(ctx)
  assert(ctx.data.model=="local")
  wraith.invocation.finish(ctx.invocation.id,{context_window=32768,capabilities={"tools"},thinking={values={false,true},default=false}})
 end,
 chat=function(ctx)
  local id=ctx.invocation.id
  local text=wraith.json.encode(ctx.data)
  assert(not text:find("private.example",1,true))
  assert(text:find("wraith::TARGET",1,true))
  assert(ctx.data.tools[1]["function"].name=="request_command")
  wraith.invocation.emit(id,{thinking="checking"})
  wraith.process.start({argv={"/bin/sh","-c","printf ready"}},function(next)
   if next.resource.final then
    assert(next.invocation.id==id)
    local output=wraith.process.output(next.resource.id).data
    wraith.invocation.finish(id,{content=output,calls={{name="request_command",arguments={command="printf wraith::TARGET",reason="inspect"}}},prompt_tokens=40,output_tokens=12})
   end
  end)
 end
})`)
	a.Config.Provider = "custom"
	a.Config.Providers["custom"] = config.Provider{Kind: "plugin", Model: "local", ActionProtocol: "native"}
	a.Variables = variables.NewSnapshot([]variables.Entry{{Name: "TARGET", Value: "private.example"}}, nil)
	client := a.providerClient(a.Config)
	var models []ollama.Model
	var info ollama.Info
	var response ollama.Response
	var streamed strings.Builder
	err := runTransport(t, a, func() (err error) {
		models, err = client.Models(t.Context())
		if err != nil {
			return err
		}
		info, err = client.Show(t.Context(), "local")
		if err != nil {
			return err
		}
		response, err = client.ChatWithRepair(t.Context(), ollama.Request{Model: "local", ExecutionMode: "agent", Options: ollama.Options{Context: 32768}, Messages: []session.Message{{Role: "user", Content: "inspect private.example"}}}, func(update provider.StreamUpdate) { streamed.WriteString(update.Chunk.Message.Content) })
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	command, err := contextual.RequestedCommand(response.Message.Content)
	if err != nil || command == nil || command.Command != "printf wraith::TARGET" || !strings.HasPrefix(response.Message.Content, "ready") || response.Message.Thinking != "checking" || response.PromptCount != 40 || len(models) != 1 || models[0].Name != "local" || info.ContextWindow != 32768 || streamed.String() != response.Message.Content {
		t.Fatalf("provider contract mismatch: models=%+v info=%+v response=%+v streamed=%q command=%+v err=%v", models, info, response, streamed.String(), command, err)
	}
}

func TestLuaProviderMalformedResponseAndFailureStayVisible(t *testing.T) {
	for _, body := range []string{
		`wraith.invocation.finish(ctx.invocation.id,{content="bad",unknown=true})`,
		`wraith.invocation.emit(ctx.invocation.id,{calls={{name="request_command",arguments={}}}})`,
		`wraith.invocation.fail(ctx.invocation.id,"backend unavailable")`,
		`wraith.invocation.finish(ctx.invocation.id,{calls={{name="request_command",arguments={}}},reason="length"})`,
	} {
		t.Run(body, func(t *testing.T) {
			a := pluginFixture(t, `wraith.providers.register("custom",{models=function() end,show=function() end,chat=function(ctx) `+body+` end})`)
			adapter := &luaProvider{runtime: a.plugins, definition: a.pluginManifest.Providers[0]}
			var response ollama.Response
			err := runTransport(t, a, func() (err error) {
				response, err = adapter.Chat(t.Context(), ollama.Request{Model: "local"}, nil)
				return err
			})
			if err == nil || len(response.Calls) > 0 {
				t.Fatalf("invalid response accepted: %+v, %v", response, err)
			}
		})
	}
}
