package ui

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/runner"
	"github.com/erdajt/wraith/internal/session"
)

func TestEmptyOrThrottledPollYieldsChatUntilCompletion(t *testing.T) {
	for _, throttled := range []bool{false, true} {
		name := "empty"
		if throttled {
			name = "throttled"
		}
		t.Run(name, func(t *testing.T) {
			a := fixture(t)
			r := runner.Record{ID: session.ID(), Session: a.Session.ID, Agent: true, ShareOutput: true, Background: true, Status: "running"}
			if throttled {
				a.lastPoll[r.ID] = time.Now()
				r.Output = "new output\n"
			}
			if err := a.history().Save(r); err != nil {
				t.Fatal(err)
			}
			calls := 0
			a.Client.HTTP.Transport = transport(func(req *http.Request) (*http.Response, error) {
				var body ollama.Request
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				calls++
				content := "```wraith-tool\n{\"tool\":\"poll\",\"id\":\"" + r.ID + "\"}\n```"
				if calls > 1 {
					last := body.Messages[len(body.Messages)-1].Content
					if !strings.Contains(last, "status: completed") {
						t.Errorf("provider resumed without completion: %s", last)
					}
					content = "completion received"
				}
				data, err := json.Marshal(ollama.Response{Done: true, Message: session.Message{Role: "assistant", Content: content}})
				if err != nil {
					t.Error(err)
				}
				return response(string(data)), nil
			})
			a.Prompt.Set("check the running job")
			a.submit()
			r.Generation = a.generation
			drain(t, a)
			if calls != 1 || a.busy || a.agentStopped || a.pendingTool != nil || a.agentReads != 1 {
				t.Fatalf("poll did not yield: calls=%d reads=%d", calls, a.agentReads)
			}
			last := a.Session.Messages[len(a.Session.Messages)-1]
			if last.Evidence == nil || last.Evidence.Kind != "waiting" || last.Evidence.ID != r.ID {
				t.Fatal("bounded waiting evidence was not saved")
			}
			other := r
			other.ID, other.Status, other.Finished = session.ID(), "completed", time.Now()
			a.pendingRuns[other.ID] = other
			a.syncJobs()
			drain(t, a)
			if calls != 2 {
				t.Fatal("silent job blocked another job's completion")
			}
			r.Status, r.Finished = "completed", time.Now()
			a.pendingRuns[r.ID] = r
			a.deliverRuns()
			drain(t, a)
			a.pendingRuns[r.ID] = r
			a.deliverRuns()
			if calls != 3 || len(a.Session.DeliveredRuns) != 2 {
				t.Fatalf("completion lost or duplicated: calls=%d", calls)
			}
		})
	}
}
