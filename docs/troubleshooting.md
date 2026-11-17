# troubleshooting

## startup and models

**the app cannot connect.** run `wraith doctor` and check the active profile in `config.yaml`. a failed plain-http loopback ollama connection can start the local server; remote endpoints and other connection errors do not. locally started server logs are in `~/.cache/wraith/ollama.log`.

**the model is missing.** `/models` lists installed ollama models or the active provider's catalog. wraith does not download weights. pull an ollama model separately or configure the api/cli model ids your account can use. `/model exact-id` bypasses the picker, not provider access checks.

**codex models are stale.** discovery uses the native cache, then a bounded app-server catalog request when needed. refresh the installed cli's catalog or configure `providers.codex.models`. a configured list replaces discovery. a custom wrapper needs the expected native protocol for automatic discovery.

**the cli exits with an error.** verify that it is installed and authenticated outside wraith. cli adapters expect structured noninteractive events and disable native tools. a wrapper that prints terminal prose instead of the expected events cannot serve as a provider. wraith starts a separate child per request; it does not attach to an existing interactive cli.

**local generation is slow.** run `ollama ps` during generation to inspect cpu/gpu placement. wraith passes model options but does not install drivers, reserve vram or force offload. lower context, output budget or thinking effort when needed; select a model that fits your available memory.

## context and thinking

**context will not fit.** `/context` reports the estimate and reserved output. `/set context NUMBER` changes the active provider/model budget; `/set context auto` removes that override. a larger number does not increase the model's native limit or available memory. reduce the draft or output reservation, compact, or start another chat with selected pins.

**compaction failed after retries.** the original session and archive remain intact. retries include the rejected result and validation error, but protected facts are never dropped just to make a replacement smaller. a full ledger can prevent further reduction. inspect `/archives` and [context](context.md) before changing the budget.

**no thinking is visible.** `/think` changes supported generation effort; `/thinking` changes display. expand a returned section with `za` or its header. a provider that does not return reasoning cannot display it. thinking can also consume the response's output budget before the answer is complete.

## keys, editor and clipboard

**enter does not send.** enter inserts a newline unless a menu or completion is active. use `ctrl+s` or normal-mode `:w` to send. if the host terminal intercepts a key, remap the named action in `keys`.

**slash opens completion instead of search.** insert-mode slash at the start of a draft opens commands. escape to normal mode for draft search, or tab to the conversation and use `/` there. `n` and `N` repeat matches.

**yank does not reach another application.** check `clipboard`. wayland defaults to `wl-copy`; x11 can use `[xclip, -selection, clipboard]`. normal-mode `p` uses the internal register even if the external helper fails. `clipboard: []` intentionally disables system copying.

**an edit did not apply.** save and exit the configured editor. quitting without saving cancels the handoff. invalid edits do not replace active config, variables or commands; the status gives a recovery file. editor commands are argument arrays, not shell expressions. configure `editor` explicitly rather than relying on `$EDITOR`.

**a command ran after saving.** saving in the approved-command editor authorizes that edited command once. saving a normal suggested block stages it in a shell; saving a prompt only returns the draft. [execution](execution.md) describes these distinct actions.

## commands and jobs

**a command appeared without an approval panel.** ordinary code is a suggestion. execution requests open approval only after validation. unresolved targets and malformed requests do not launch a process. use `/blocks` to choose a normal block manually.

**the chat stays available during a command.** this is the default in agent mode. `/jobs` opens current-session jobs and `/history` opens saved output. the agent receives a start receipt and bounded completion evidence. background jobs still end when wraith exits.

**the agent cannot inspect a job.** output tools are restricted to shared jobs in the current conversation. explicit hide-output approval denies reads. use `ctrl+g` in that output reader to share it. blocks-mode and scratch sharing is otherwise off unless `execution.auto_share` is enabled.

**the agent paused after polling.** an empty or throttled poll yields until completion or a new prompt. read and command budgets also stop automatic continuation. completion delivery waits while you type, select, edit, use a shell or review a popup, so it does not replace your draft.

**the command needs a password or full-screen interface.** tracked commands are fresh noninteractive shell jobs. use a scratch terminal for programs that require a tty. pane `cd` and shell variables do not change tracked jobs; configure `terminal.directory` and `/env` when appropriate.

**output has a gap.** capture retains a bounded prefix and independent recent tail. large output continues draining; omitted byte ranges are reported. increasing limits only affects new jobs. `/history` contains the full retained record, which can be larger than the excerpt shared with the agent.

## private variables

**the wrong scope is visible.** active precedence is global, then directory, then conversation. `/env conversation`, `/env directory` and `/env global` inspect each scope. changing `environment.scope` affects new assignments; it does not move existing entries.

**a shell has an old value.** existing scratch shells retain their starting environment. restart the pane after changing bindings. approvals freeze their bindings at review time, so later edits cannot silently change an already approved command.

**a marker keeps retrying.** inspect its name in `/env`. names are case-sensitive and follow `[A-Z_][A-Z0-9_]*`. the model gets a correction and up to three retries. exhausted responses remain visible without automatic execution. non-shell source and unsupported shell syntax may require editor review.

**the privacy journal is missing or corrupt.** outbound requests stop rather than forgetting old private values. restore the variable directory from a consistent backup. do not delete `state.json` as a connection workaround. [private variables](private-variables.md) explains retained values and exact-match redaction limits.

## sessions and display

**resume cannot find a chat.** change to the directory where it began. `/resume`, `--resume` and `wraith sessions` use canonical directory ownership. an id does not bypass that check. legacy records without a directory can still be exported.

**another writer owns the data directory.** close the other instance or use separate `--data-dir`, `--config-dir` and `--cache-dir` paths. do not remove a live writer's lock.

**the interface is cramped.** the minimum is 50 columns by 18 rows. use a larger terminal for long commands, tables and two panes. the side pane stops above the footer. reduce `terminal.split_width` if the conversation needs more width.

**transparency or fonts look different.** wraith controls cell colors and can inherit the host background. actual opacity, blur and fonts belong to the terminal/compositor. an inherited background does not make an opaque host transparent. screenshots use the documented terminal setup, not a claim that every host looks identical.

**a scratch shell keeps restarting.** `exit` starts a replacement. repeated immediate failures stop after the restart limit. fix `shell` or `terminal.directory`, then use `/terminal-restart 1` or `/terminal-restart 2`.

## useful checks

```sh
wraith --version
wraith config
wraith doctor
wraith models
wraith sessions
wraith --demo
```

report the failing action, expected result, terminal dimensions, provider kind and relevant config without credentials. include whether the same issue occurs in `--demo`. avoid sharing private command history or variable files unchanged.
