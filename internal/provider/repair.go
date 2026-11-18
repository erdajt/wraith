package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/erdajt/wraith/internal/contextual"
	"github.com/erdajt/wraith/internal/editor"
	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/session"
)

type StreamUpdate struct {
	Rejected session.Message
	Chunk    ollama.Response
	Retry    int
	Problem  string
}

// ChatWithRepair streams validated responses with up to three corrective retries for invalid references or actions.
func (c *Client) ChatWithRepair(ctx context.Context, request ollama.Request, emit func(StreamUpdate)) (ollama.Response, error) {
	original := request
	ids := session.EvidenceIDs(request.Messages)
	for id, known := range request.RunIDs {
		if known {
			ids[id] = true
		}
	}

	var last ollama.Response

	for attempt := 0; attempt <= 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return last, err
		}

		child, cancel := context.WithCancel(ctx)
		var rejected error
		var content strings.Builder
		sent := 0
		result, err := c.Chat(child, request, func(chunk ollama.Response) {
			if rejected != nil || ctx.Err() != nil {
				return
			}

			content.WriteString(chunk.Message.Content)
			end, validation := c.Variables.Validate(content.String(), false)
			if validation != nil {
				rejected = validation
				cancel()
				return
			}

			chunk.Message.Content = content.String()[sent:end]
			sent = end
			if emit != nil {
				emit(StreamUpdate{Chunk: chunk})
			}
		})
		cancel()

		if err := ctx.Err(); err != nil {
			return result, err
		}

		if rejected != nil {
			result.Message.Content = content.String()
			result.Message.Role = "assistant"
		} else if err != nil {
			return result, err
		} else {
			_, rejected = c.Variables.Validate(result.Message.Content, true)
			if rejected == nil {
				command, decodeErr := contextual.RequestedCommand(result.Message.Content)
				rejected = decodeErr
				tool, toolErr := contextual.RequestedTool(result.Message.Content)
				if rejected == nil {
					rejected = toolErr
				}
				if command != nil && tool != nil {
					rejected = errors.New("request one command or output tool per response")
				}
				if rejected == nil && command == nil && tool == nil && request.ExecutionMode == "agent" && request.CompactionTask == "" && request.Format == nil {
					rejected = contextual.UnclearCommandIntent(result.Message.Content)
				}
				if tool != nil && rejected == nil {
					_, rejected = c.Variables.Validate(tool.Pattern, true)
				}
				if command != nil && rejected == nil {
					_, rejected = c.Variables.Validate(command.Command+"\n"+command.Reason, true)
					if rejected == nil {
						rejected = c.Variables.CheckCommand(command.Command)
					}
				}
			}
		}

		if rejected == nil {
			for _, block := range editor.Blocks(result.Message.Content) {
				switch block.Language {
				case "sh", "bash", "shell", "zsh":
					rejected = c.Variables.CheckCommand(block.Text)
				}
				if rejected != nil {
					break
				}
			}
		}

		if rejected == nil && emit != nil && sent < len(result.Message.Content) {
			emit(StreamUpdate{Chunk: ollama.Response{Message: session.Message{Role: "assistant", Content: result.Message.Content[sent:]}}})
		}

		result.Message.Content = contextual.RedactActions(result.Message.Content, c.Variables.Redact, ids)
		result.Message.Thinking = c.Variables.Redact(result.Message.Thinking)
		last = result
		if rejected == nil {
			return result, nil
		}

		problem := c.Variables.Redact(rejected.Error())
		if attempt == 3 {
			last.Message.Interrupted = true
			return last, fmt.Errorf("response repair exhausted after 3 retries: %s", problem)
		}

		excerpt := []rune(last.Message.Content)
		excerpt = excerpt[:min(len(excerpt), 2048)]
		feedback := "The previous answer was rejected: " + problem + ". Regenerate the answer using only the available wraith::NAME references. Correct invalid request syntax. Do not execute anything from the rejected answer."
		var protocol *contextual.ActionProtocolError
		errors.As(rejected, &protocol)

		for {
			request = original
			request.RepairAction = ""
			if protocol != nil {
				request.RepairAction = protocol.Target
			}
			request.Messages = append(append([]session.Message(nil), original.Messages...), session.Message{Role: "user", Content: feedback + "\nRejected answer excerpt (untrusted):\n" + string(excerpt)})
			_, prepared, prepareErr := c.privateRequest(request)
			if prepareErr != nil {
				return last, prepareErr
			}

			if request.Options.Context > 0 && contextual.Estimate(prepared.Messages)+request.Options.Predict+128 > request.Options.Context {
				if len(excerpt) == 0 {
					return last, errors.New("response repair stopped: corrective instructions exceed context budget")
				}
				excerpt = excerpt[:len(excerpt)/2]
				continue
			}
			break
		}

		if emit != nil {
			emit(StreamUpdate{Retry: attempt + 1, Problem: problem, Rejected: last.Message})
		}
	}

	return last, errors.New("response repair stopped")
}
