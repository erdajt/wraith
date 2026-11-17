# private variables

private variables let a model refer to a target by name while wraith keeps the value local. the model receives names and attached descriptions, uses `wraith::NAME` in prose or commands, and sees the same markers in shared command output. the local transcript and approval preview substitute values in green.

## create and manage

```text
/env
/env edit
/env set TARGET_DOMAIN=lab.example
/env unset TARGET_DOMAIN
/env conversation
/env directory
/env global
/env global edit
```

a bare `TARGET_DOMAIN=lab.example` in the prompt also creates a binding in the configured scope. assignments and `/env` commands are excluded from input recall. `/env` shows local values and descriptions; enter opens details, `n` or `e` opens the default editor, `d` removes a binding, and `g` opens the conversation/directory/global scope picker. these bindings use `keys.env_new`, `env_edit`, `env_remove` and `env_scope`.

choose the default storage through `/settings.environment` → `environment.scope`, or set it directly:

```text
/set environment.scope conversation
/set environment.scope directory
/set environment.scope global
```

| scope | persistence |
| --- | --- |
| `conversation` | only this saved conversation, including resume |
| `directory` | all chats launched in the same canonical directory; default |
| `global` | always keep across conversations and directories |

changing the setting affects future assignments and default editor selection. existing bindings are not copied or moved. `/env conversation`, `/env directory` and `/env global` manage a scope explicitly without changing the default. conversation bindings override directory bindings, which override global bindings with the same name. new chats, forks and archive restores start with no conversation bindings; their directory/global defaults remain available. a chat containing only conversation variables is saved so it can be resumed.

directory bindings use the canonical launch directory. global changes require confirmation because they affect every directory. removing a directory override reveals any global binding. saving in the editor validates the complete file; an invalid edit or stale revision leaves the active configuration unchanged and retains the editor file for repair.

```dotenv
# domain for the isolated lab
TARGET_DOMAIN=lab.example

// application under review
TARGET_SUBDOMAIN=app.lab.example // use for application checks

/* callback service for the lab */
CALLBACK_URL="https://callback.lab.example/path#fragment"
```

comments attach on the same line or directly above an assignment. a blank line detaches preceding comments. `#`, `//` and `/* ... */` are accepted outside quotes when preceded by whitespace or at the start of a line. urls containing `://` are preserved. block comments may span lines but cannot nest. comments are rendered in the picker and included in the assistant's name inventory after redaction.

names match `[A-Z_][A-Z0-9_]*`, at most 64 bytes. each scope allows 128 bindings, values up to 16 kib and descriptions up to 4 kib, within a 256 kib file. values must be nonempty and contain no control characters or reserved `wraith::` prefix. quoted values are literal; double quotes permit only escaped quotes and backslashes. no shell interpolation, command substitution or dotenv expansion is performed.

## commands and model requests

approved shell commands resolve markers as literal arguments, assignments or supported redirection words. the shell parser rejects unsafe locations such as an executable name, heredoc or ambiguous expansion. metacharacters inside a value cannot become new shell syntax. copying a shell block resolves private arguments with shell quoting while the stored conversation keeps markers. approval freezes the resolved command and bindings; editing sees plain command text, and saving authorizes that edited command.

new scratch shells and approved tracked jobs receive active bindings in their environment. existing scratch shells retain their original environment until restarted. non-shell source blocks resolve references for editor review rather than guessing the source language's quoting rules.

provider requests, shared output and compaction inputs pass through the privacy boundary. registered literal values and supported JSON, URL and shell-quoted forms are replaced with markers. old bindings remain in a local redaction journal after removal or rotation. model-facing searches operate on redacted output. CLI provider processes have registered names and environment values containing private literals removed; conflicting authentication or routing configuration fails locally rather than silently changing credentials.

this is exact-value redaction, not general anonymization: arbitrary encodings, substrings, tool transformations and partial terminal selections can evade matching. comments are intended to be shared, so describe purpose without adding unrelated secrets. local files, editor buffers, command history may contain values; both CLI and in-app exports redact registered values by default. compression is not encryption.

## invalid references and retries

text streams as it arrives. a marker split across chunks is buffered only until its name can be validated. unknown names, malformed references and invalid execution requests cancel that attempt. the UI shows `retrying 1/3` with the validation reason while the replacement response streams.

corrective requests include the original task, a bounded rejected excerpt, the failure and the valid inventory. there are at most three retries after the initial attempt. exhausted responses remain visible as interrupted text with no automatic actions; unresolved commands can be edited manually. transport failures and cancellation are not retried as variable mistakes.

superseded attempts are compressed under `~/.cache/wraith/repairs/archives/`, separate from accepted chat history. they are not automatically pruned. variables live under `~/.config/wraith/variables/`: `global.env`, `directories/<directory-hash>.env`, `conversations/<session-id>.env` and `state.json` for accepted/retained bindings. back up this directory together; a missing or corrupt journal stops outbound requests until repaired so forgotten values are not silently exposed.

approvals display the frozen, shell-quoted command and its reason. changing a variable while approval is open does not change the displayed or executed binding.
