# providers

`/providers` chooses a configured backend. selecting a profile opens its model catalog unless it has a saved default. `/models` opens the active provider's model picker; `/model exact-model-id` selects an id directly. a session records its provider and model, including when resumed.

| kind | transport | model discovery |
| --- | --- | --- |
| `ollama` | native streaming chat | installed models from `/api/tags` |
| `openai` | openai-compatible chat completions and sse | `/models`, or configured ids |
| `anthropic` | messages api and sse | `/models`, or configured ids |
| `codex-cli` | installed `codex exec` with json events | configured ids, plus `default` when no catalog is set |
| `claude-cli` | installed `claude -p` with streaming json | configured ids, plus `default` when no catalog is set |

built-in profiles are `ollama`, `openai`, `anthropic`, `deepseek`, `codex` and `claude`. add any number of named compatible endpoints. model availability and thinking capabilities vary by account and model; configured catalogs are explicit ids, not a claim that the service grants access.

## api profiles

edit `~/.config/wraith/config.yaml` through `/settings` → model → providers, or with your editor:

```yaml
provider: openai
model: your-model-id
providers:
  openai:
    kind: openai
    base_url: https://api.openai.com/v1
    key_env: OPENAI_API_KEY
    model: your-model-id
    token_field: max_completion_tokens
  private-server:
    kind: openai
    base_url: http://127.0.0.1:8000/v1
    models: [your-local-model]
    token_field: max_tokens
    system_prompt: prefer concise reproducible checks
```

set the named environment variable before launching wraith. `api_key` can hold a key directly in the private config file; HTTP profiles prefer a nonempty environment value over an inline key. `headers` supports provider-specific headers. credentials are not shown in settings summaries or copied into conversation context. protect exports and config backups yourself; compression is not encryption.

`extra` adds endpoint-specific request fields, such as temperature or supported reasoning settings. it cannot replace model, messages, system, streaming or tool-control fields. `token_field` chooses `max_tokens` or `max_completion_tokens` for compatible endpoints. the app's context capacity governs local budgeting; cloud APIs enforce their own model limits and do not receive Ollama's `num_ctx` option.

`thinking` lists supported picker values for a profile. `thinking_mode` can be `reasoning_effort`, `thinking`, or `adaptive` where the backend supports it. do not list values unsupported by the selected model. an empty list leaves `auto` available. Ollama discovers these values from the model instead.

`system_prompt` adds provider-specific instructions to ordinary chat requests. compaction uses its dedicated extraction and audit instructions without this chat prompt. global user preferences remain in `instructions.md`.

```sh
wraith --provider openai --model your-model-id
wraith --provider codex
```

## cli profiles

install and authenticate the CLI separately. the default profiles reuse its saved login. `command` is an executable-and-arguments array, allowing an absolute path or your own wrapper. configure `models` with ids supported by that installation; `default` omits the model flag and uses the CLI default.

wraith launches an isolated, noninteractive child for each request, sends the active conversation through stdin and reads structured response events. it does not attach to another interactive terminal or import an unrelated CLI conversation. Wraith owns session persistence and compaction, so archived turns do not silently return through an independent CLI history.

Codex uses an ephemeral, read-only invocation with shell tools disabled and without user config. Claude uses safe mode with built-in and MCP tools disabled. both start in a temporary directory, and Wraith supplies its system instructions. this keeps command execution in Wraith's approval flow; it is not a replacement for the native CLI's full agent interface. custom wrappers must preserve the expected event format and these execution boundaries.

streamed text and exposed reasoning appear in the same conversation UI as Ollama. private reasoning that a provider does not return cannot be displayed. cancellation terminates the child process group. CLI flags evolve; unsupported versions report errors rather than falling back to unstructured terminal scraping.

## protocol references

- [openai chat completions](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create)
- [anthropic message streaming](https://platform.claude.com/docs/en/build-with-claude/streaming)
- [deepseek chat completions](https://api-docs.deepseek.com/api/create-chat-completion/)
- [codex noninteractive mode](https://learn.chatgpt.com/docs/non-interactive-mode)
- [codex configuration](https://learn.chatgpt.com/docs/config-file/config-reference)
- [claude code programmatic usage](https://code.claude.com/docs/en/headless)
- [claude code cli options](https://code.claude.com/docs/en/cli-reference)

for compatible APIs, `reasoning_effort` mode maps `off` to `none` and `on` to `medium`; explicit strings pass through. `thinking` mode sends an enabled/disabled object and includes `reasoning_effort` when a graded value is selected. advertise only values supported by the selected model. [deepseek thinking configuration](https://api-docs.deepseek.com/guides/thinking_mode/)
