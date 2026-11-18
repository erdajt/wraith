package provider

import (
	"errors"
	"testing"

	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/contextual"
	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/session"
)

func TestBudgetMatchesPreparedTransportForEachProtocol(t *testing.T) {
	for _, kind := range []string{"ollama", "openai", "anthropic"} {
		for _, mode := range []string{"agent", "blocks"} {
			for _, protocol := range []string{"native", "text", "auto"} {
				t.Run(kind+"/"+mode+"/"+protocol, func(t *testing.T) {
					c := config.Default()
					c.Provider, c.Execution.Mode = kind, mode
					p := c.Providers[kind]
					p.ActionProtocol = protocol
					c.Providers[kind] = p
					client := New(c)
					req := ollama.Request{ExecutionMode: mode, PrivateInventory: true, Capabilities: []string{"tools"}, Messages: contextual.ForChat(session.New("test"), c, "preserve this fact"), Options: ollama.Options{Predict: c.Predict}}
					input, err := client.Estimate(req)
					if err != nil {
						t.Fatal(err)
					}
					req.Options.Context = input + c.Predict + 128
					_, prepared, err := client.privateRequest(req)
					if err != nil || !requestFits(prepared) {
						t.Fatalf("exact-fit request rejected: %v", err)
					}
					prepared.Options.Context--
					if requestFits(prepared) {
						t.Fatal("overflow accepted by transport")
					}
					var overflow *ContextError
					if err := CheckContext(input, c.Predict, prepared.Options.Context); !errors.As(err, &overflow) {
						t.Fatalf("missing typed overflow: %v", err)
					}
				})
			}
		}
	}
}
