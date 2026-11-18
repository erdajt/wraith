# providers

`/providers` chooses a configured backend. selecting a profile opens its model catalog unless it has a saved default. `/models` opens the active provider's model picker; `/model exact-model-id` selects an id directly. a session records its provider and model, including when resumed.

| kind | transport | model discovery |
| --- | --- | --- |
| `ollama` | native streaming chat | installed models from `/api/tags` |
| `openai` | openai-compatible chat completions and sse | `/models`, or configured ids |
| `anthropic` | messages api and sse | `/models`, or configured ids |
| `codex-cli` | installed `codex exec` with json events | native cached catalog, then app-server `model/list`; configurable ids |
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

`extra` adds endpoint-specific request fields, such as temperature or supported reasoning settings. it cannot replace model, messages, system, streaming or tool-control fields. `token_field` chooses `max_tokens` or `max_completion_tokens` for compatible endpoints. the app's context capacity governs local budgeting; cloud APIs enforce their own model limits and do not receive ollama's `num_ctx` option.

`thinking` lists supported picker values for a profile. `thinking_mode` can be `reasoning_effort`, `thinking`, or `adaptive` where the backend supports it. do not list values unsupported by the selected model. an empty list leaves `auto` available. ollama discovers these values from the model instead. codex discovers its reasoning values from native catalog metadata when no explicit thinking list is configured.

`system_prompt` adds provider-specific instructions to ordinary chat requests. compaction uses its dedicated extraction and audit instructions without this chat prompt. global user preferences remain in `instructions.md`.

```sh
wraith --provider openai --model your-model-id
wraith --provider codex
```

## action transport

`providers.<name>.action_protocol` accepts `auto`, `native` or `text`; an omitted value means `auto`.

- `auto` uses native function calls for ollama models that advertise tools. other providers keep the text request protocol.
- `native` enables function calls for ollama, openai-compatible or anthropic endpoints. select a model that supports tools; unsupported endpoints report their error without a silent fallback.
- `text` uses the existing fenced request protocol. cli providers always use this transport and reject `native`.

both transports use the same approval window. agent mode offers command requests and bounded output reads. blocks mode offers command requests only. compaction and structured extraction requests offer no tools. tool schemas count toward the request budget.

native arguments stay private to the decoder until a completed response passes validation. multiple calls, mixed text/native requests and malformed arguments are rejected. corrective retries are visible and stop after three attempts. network and authentication failures are not retried as protocol corrections.

calls are converted to ordinary conversation text, and job receipts/results return as user evidence. provider call ids, native tool-result envelopes and signed thinking blocks are not replayed. wraith's durable job ids remain valid across compaction and resume. historical calls never execute on resume.

## cli profiles

install and authenticate the CLI separately. the default profiles reuse its saved login. `command` is an executable-and-arguments array, allowing an absolute path or your own wrapper. `/models` opens the codex catalog and supports `j`/`k`, arrows, filtering and enter. choosing a model saves it as that provider's default and refreshes its context and thinking settings. `/model exact-model-id` also selects directly.

codex discovery reads `$CODEX_HOME/models_cache.json`, or `~/.codex/models_cache.json`. hidden models are excluded. when the cache is absent or unusable, wraith starts the native app server for a bounded `model/list` request; it does not start a conversation or generation. cached entries reflect the last codex catalog refresh and do not guarantee current account access. run codex itself to refresh stale metadata.

an explicit nonempty `models` list replaces discovery. an old codex list containing only `default` now enables discovery; `default` remains available through `/model default`, where it omits the model flag. claude continues to use configured ids, or `default` when none are configured. custom codex wrappers need the app-server protocol for discovery without a cache; otherwise configure their model list.

wraith launches an isolated, noninteractive child for each request, sends the active conversation through stdin and reads structured response events. it does not attach to another interactive terminal or import an unrelated CLI conversation. wraith owns session persistence and compaction, so archived turns do not silently return through an independent CLI history.

codex uses an ephemeral, read-only invocation with shell tools disabled and without user config. claude uses safe mode with built-in and MCP tools disabled. both start in a temporary directory, and wraith supplies its system instructions. this keeps command execution in wraith's approval flow; it is not a replacement for the native CLI's full agent interface. custom wrappers must preserve the expected event format and these execution boundaries.

streamed text and exposed reasoning appear in the same conversation UI as ollama. private reasoning that a provider does not return cannot be displayed. cancellation terminates the child process group. CLI flags evolve; unsupported versions report errors rather than falling back to unstructured terminal scraping.

## protocol references

- [openai chat completions](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create)
- [anthropic message streaming](https://platform.claude.com/docs/en/build-with-claude/streaming)
- [deepseek chat completions](https://api-docs.deepseek.com/api/create-chat-completion/)
- [codex noninteractive mode](https://learn.chatgpt.com/docs/non-interactive-mode)
- [codex configuration](https://learn.chatgpt.com/docs/config-file/config-reference)
- [claude code programmatic usage](https://code.claude.com/docs/en/headless)
- [claude code cli options](https://code.claude.com/docs/en/cli-reference)

for compatible APIs, `reasoning_effort` mode maps `off` to `none` and `on` to `medium`; explicit strings pass through. `thinking` mode sends an enabled/disabled object and includes `reasoning_effort` when a graded value is selected. advertise only values supported by the selected model. [deepseek thinking configuration](https://api-docs.deepseek.com/guides/thinking_mode/)

## profile reference

| field | purpose |
| --- | --- |
| `kind` | `ollama`, `openai`, `anthropic`, `codex-cli` or `claude-cli` |
| `base_url` | endpoint for http backends; ollama falls back to the root `host` |
| `model` | saved default selected when activating this profile |
| `models` | explicit model ids; a nonempty list replaces discovery |
| `key_env` | process environment variable containing the api key |
| `api_key` | inline key when environment credentials are not used |
| `headers` | extra http headers |
| `command` | executable and arguments for cli profiles |
| `context` | provider budget override; zero uses automatic resolution |
| `model_contexts` | exact model ids mapped to positive capacities |
| `thinking` | supported values offered by the picker |
| `thinking_mode` | backend request mapping for the configured effort |
| `token_field` | compatible-api token budget field |
| `system_prompt` | additional instructions for ordinary chat |
| `extra` | permitted endpoint-specific request fields |

profile names are local labels. for example, two compatible endpoints can use different names with the same `kind: openai`. changing providers preserves the conversation, so the next request sends the retained context to that provider after privacy filtering. changing a model resets thinking to `auto`; its own capacity override is restored if configured.

## thinking and display

`/think` lists values supported by the active model/profile. `auto` leaves the backend default intact. `/thinking` and `appearance.thinking` control visibility, independently of generation effort. `za` or a click expands a returned thinking section. reasoning streams separately from the answer and is saved for local display; it is not replayed as ordinary conversation text.

local startup and model switches run discovery/capability checks and preload the selected ollama model. api and cli profiles check their configured backend without downloading local weights. authenticated provider calls are not exercised by the offline fixture tests.
