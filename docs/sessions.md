# sessions and recovery

plain `wraith` creates a new conversation in the current directory. `/clear` and `/new` do the same inside the app, retaining the old session. there is no `--new` flag.

## resume

```sh
wraith --resume
wraith --resume SESSION_ID
wraith sessions
```

```text
/resume
/resume SESSION_ID
/chats
/sessions
```

without an id, resume opens a picker. use arrows or `j/k`, `/` to filter, and enter to select. only conversations from the current canonical directory appear. launching through a symlink to the same directory still matches.

an invalid id reports `invalid session id` and starts fresh. an id from another directory reports where the original conversation occurred and starts fresh. change to that directory before resuming. sessions without a recorded directory can be exported but are not assigned to a directory implicitly.

resume restores the recorded provider and model. explicit startup `--provider` and `--model` flags can override them. the profile must still exist in the config. conversation-scoped variables return with their session; directory and global variables follow their own scopes.

## unsent drafts

`:wq` saves the exact prompt and quits. resuming restores it into the input field without sending it. drafts are stored separately from messages and are excluded from model requests. a positional startup prompt overrides the restored draft, including when selecting a session from the resume picker.

`:q` closes one conversation window or exits the last. `:qa` exits all windows. a nonempty prompt blocks either operation; `:q!` and `:qa!` discard it explicitly. `:w` and `:w!` send the prompt. save failures keep the input and windows open. submitting a saved draft removes its stored copy in the same write that adds the user message.

## edit and fork

select a user message in the conversation and press `ctrl+e`. the confirmation explains that editing forks the chat at that point. accepting creates a new session with the earlier messages and places the selected prompt in the input. later messages are absent from the fork; the original session retains them.

editing an assistant message uses the configured external editor. the previous session is archived before the replacement is saved. saving assistant text changes local conversation content; it does not rerun the model automatically.

new chats, forks and archive restores receive new ids and do not copy conversation-scoped variables. inherited directory/global bindings remain available.

## interrupted responses

received text is saved when generation is interrupted. `/retry` repeats an interrupted request; `/cancel` stops model work and invalidates pending approval. rejecting a model command stops that agent turn. tracked commands have separate lifecycle and recovery rules in [execution](execution.md).

## export

```text
/export
/export /tmp/review.md
```

```sh
wraith export SESSION_ID > review.md
```

exports are markdown and redact registered private values. they contain the active context, including compacted state. use an archive when you need the full pre-compaction transcript. exporting does not mutate the saved session or grant permission to resume it from another directory.

## archives

`/archives` lists originals for the current conversation. choose one to restore it into a new session. `/restore filename.json.gz` restores a specific archive. the original archive remains unchanged.

compaction writes both an immutable archive and a recovery cache before asking for a summary. a failed replacement leaves the original active. assistant edits also archive the source first. see [context](context.md) for the transaction, retry policy and preservation limits.

## storage and backups

| root | what to keep |
| --- | --- |
| `~/.local/share/wraith` | sessions, archives, command records, input history, snippets and exports |
| `~/.config/wraith` | config, instructions, scoped variables and retained-value redaction journal |
| `~/.cache/wraith` | recovery copies, failed edits, superseded replies and local ollama log |

back up config and data together if you want to preserve private-variable redaction across old conversations. losing the retained-value journal can prevent requests from safely proceeding. files may contain local values even when provider-bound text is redacted.

stop wraith before copying the complete storage tree. one interactive writer owns a data directory. `--config-dir`, `--data-dir` and `--cache-dir` can isolate separate installations or test workspaces. gzip files use private permissions; they are not encrypted. archives and command history are not pruned automatically.
