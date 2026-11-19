# commands and evidence

`execution.mode` selects `agent` or `blocks`. both distinguish concrete execution requests from ordinary markdown examples. neither runs a command merely because it appeared in a response.

in agent mode, approved commands return their input, status and bounded output to the model, which can continue the task. blocks mode keeps the approval flow but shares output only when requested, unless `execution.auto_share` is enabled. an explicit hide-output choice overrides either mode.

## approval

execution requests display the resolved command and its `reason:`. transport markers and json fields are decoded before chat rendering, selection, copying or editor handoff. unresolved placeholders remain suggestions. read the full command before approving; placeholder detection cannot recognize every possible template.

in agent mode, a command object in an ordinary json block or a misplaced request marker triggers a visible correction attempt, up to three retries. the model must clarify whether it wants execution or an example. wraith never promotes the malformed answer itself. plain shell examples remain suggestions; open `/blocks` to act on one manually.

choose with `j/k` or arrows and enter:

- **run** executes once and captures merged stdout/stderr; no second approval follows.
- **run · hide output from agent** shares only the redacted command and completion status. the model cannot poll or search this run's output.
- **edit** opens the resolved command in your configured editor. save and exit to execute the edited command once. exiting without saving runs nothing. invalid edits remain available for recovery.
- **view full command** opens a scrollable preview with the reason.
- **ask assistant to modify** sends an inline revision. an empty revision field prepares a `modify:` draft in the main prompt, without sending it.
- **reject** stops the requesting agent turn and cancels jobs requested by that agent turn. manually launched background jobs keep running even when their output is shared with the agent. rejecting a manually selected block does not cancel unrelated work. `ctrl+c` and `/cancel` invalidate pending approvals, including full previews and modification dialogs; an interrupted scratch shell still receives its own `ctrl+c`.

approval freezes the session, turn, directory, shell, timeout, private bindings and sharing policy. later setting changes cannot change that launch. an approval from a replaced session or turn cannot execute.

## ordinary blocks

`<leader>b` or `/blocks` lists code blocks. enter on a command under the conversation cursor opens it in your editor; enter on an assistant message offers its blocks. selecting run authorizes a tracked run directly.

saving a shell block inserts its text automatically into an empty integrated zsh edit buffer. enter then executes it; saving alone never does. an occupied input buffer keeps the snippet staged. `ctrl+c` clears shell input, and enter retries the empty-buffer check. other shells keep manual staging: enter inserts and the next enter executes. manual multiline insertion requires bracketed paste support.

while insertion is pending, escape requests cancellation and `ctrl+c` also interrupts the shell. an uncertain acknowledgement blocks reinsertion until the shell replies or is restarted. after confirmed staging, `r` offers a tracked run, `e` edits again, and escape dismisses staging. source languages such as python and go use a saved file and interpreter. private references in non-shell source require editor review before running; wraith does not infer language-specific quoting. see [terminal recovery](terminals.md#saved-command-insertion).

## background jobs and history

tracked jobs use the configured shell executable with `-c` in `terminal.directory`, or the launch directory when empty. each job starts a fresh shell with the active private bindings. use a scratch pane for persistent interactive state or programs needing a tty.

agent mode runs every approved command as a tracked background job, even if the model omits `background` or sets it to false. approval shows this before execution. launching keeps the chat and draft available; it does not open an output popup. the agent receives a job id and can poll its status or search bounded output while it runs. blocks mode retains its existing foreground behavior unless background execution is explicitly requested.

`/jobs` or `<leader>j` shows current-session jobs; `/history` shows saved command records. open a job to view live output, status and exit code. `ctrl+x` cancels the selected job, configurable as `keys.job_cancel`. completion evidence waits while you type, edit or select output. it stays in its original conversation and is delivered once.

a start receipt means the job was accepted; process startup may still fail and produce a failed result. polls include status and new complete output lines. an empty or throttled poll saves a waiting status and pauses automatic model calls, leaving the chat available. completion or a new user prompt resumes work. hidden-output receipts do not offer polling.

output readers support vim motions, counts, `/` search, visual selection and yanks. selection freezes the visible snapshot so incoming output does not move it. `r` refreshes it. live views show the recent tail; completed views include the retained prefix and tail with gap offsets.

completion evidence waits while you edit, type a draft, select text, use a shell or review a popup. it joins the next safe request without replacing your draft. session delivery ids prevent duplicate insertion after resume. jobs from an older turn do not independently restart that turn.

## output access

agent-mode tracked runs share by default. blocks-mode runs share only through `ctrl+g` in the output viewer or `execution.auto_share: true`. explicitly hidden output remains private until you choose to share it. the manual binding appears only when sharing is needed. sharing a record twice does not duplicate evidence; cancelled runs can be shared without automatically continuing.

completion evidence uses a bounded excerpt, reserving space for the retained tail when capture overflowed. the model can poll new lines or search authorized output by literal text or regular expression, then read bounded pages with source line numbers. tools only access shared jobs in the current session. matching occurs after private-value redaction. withheld jobs cannot be queried.

read pages contain at most 200 lines and 16 kib before the smaller configured context budget applies. queries allow at most ten context lines before/after a match and a 4096-byte pattern. `ignore_case` supports case-insensitive literal or regex matching. `from` and the returned `next` cursor are one-based output line numbers; omitted or zero `from` starts at the first line. omitted or zero `limit` uses 80 lines. byte ranges describe capture gaps only. pages include observed/prefix/tail bytes, retained/matched line counts, elapsed milliseconds and omitted byte ranges. oversized lines are marked omitted so polling still advances. when context cannot hold more evidence, reads wait for compaction without advancing their cursor or consuming their budget. polls are limited to one per second per job; their cursor advances only after the result is saved. full retained output remains available locally.

`execution.max_steps` limits approved model commands per user turn; `execution.max_reads` limits output queries. reaching either pauses continuation. a new prompt resets these budgets. all command output is untrusted evidence, not new instructions.

in scratch copy mode, `ctrl+g` shares the selection or snapshot. `execution.auto_share` includes changed snapshots before the next request. snapshots cannot reliably identify command boundaries, exit status or discarded scrollback; use tracked jobs when those matter.

## limits and recovery

model output reads run outside the ui loop and share two source-reader slots with extension workspace queries. saved records have a 64 mib decoded-source cap. a read keeps its original session, turn and sharing policy; a changed privacy snapshot requires a new read. typing defers delivery without advancing the polling cursor. cancellation, session changes and runtime replacement discard stale results. trusted external edits to history files are outside this in-app ownership guarantee.

output searches merge retained prefix and tail lines without indexing the entire capture. context windows keep at most ten preceding lines; result pages contain at most 200 lines and 16 kib. overlapping tail lines replace the corresponding prefix lines. coverage and match counts still cover all retained complete lines.

searches stop on cancellation or after ten seconds. regex patterns accept at most 4,096 source bytes and 4,096 expanded program instructions. total regex work is limited to 67,108,864 instruction-times-text units; exceeding it returns an error rather than incomplete search results. use a literal pattern when a complex regex exceeds the work limit. these limits apply to local extension queries and model-facing searches; output-sharing rules remain separate.

| setting | default | accepted values |
| --- | --- | --- |
| `execution.max_jobs` | `4` | 1–32 concurrent jobs |
| `execution.command_timeout` | `10m` | positive Go duration |
| `execution.capture_bytes` | `8388608` | 64 kib–256 mib retained prefix |
| `execution.tail_bytes` | `65536` | 4 kib–1 mib, no larger than the prefix limit |
| `execution.output_context` | `8192` | 256–65536 bytes, further bounded by context |
| `execution.max_steps` | `8` | 1–100 model commands per turn |
| `execution.max_reads` | `16` | 1–256 output queries per turn |

jobs drain output after the prefix fills, keeping an independent recent tail. initial intent is saved before spawning. live journals checkpoint each second or 256 kib, then finalize into compressed history. a crash can lose output since the last checkpoint. recovered incomplete jobs are marked interrupted and never rerun automatically. another running instance's locked journals are left alone.

quitting, cancellation and timeout terminate owned process groups; background jobs do not survive a normal app exit. commands that deliberately detach can escape process-group cleanup. a failed final save leaves the result visible in memory with a persistence error. compression and private file permissions do not encrypt history.

approval opens a focused panel with a bright border, a filled selection row, and separate command and reason sections. the chat and scratch panes are hidden behind it while the decision is pending. the panel respects inherited terminal backgrounds. long commands remain available through **view full command**, and all actions remain reachable with `j/k` or arrow keys at the minimum supported terminal size.

if the model sends an output query through the command channel, wraith rejects it without executing anything and supplies a specific correction on retry. the same applies to a command sent as an output query. internal block names appear only in model-facing feedback, while the status explains the mismatch in plain language. the existing three-retry limit still applies.

## mode instructions

wraith builds each request with one execution policy selected from the current setting. switching modes replaces that policy on the next request, including in an existing conversation. both policies include their own command-request format. only agent mode includes the output-query and polling protocol.

agent mode tells the model to request approval when the user asks it to perform a task. requests to explain or show a command still produce suggestions. blocks mode favors inspectable shell blocks and permits explicit approval requests, without autonomous output polling. configured private references count as available inputs in either mode.


## lua tools and command policies

plugins can register schema-validated model tools through `wraith.tools.register`. tool calls use native approval and tracked background execution; their output follows the selected sharing decision. [tool contracts](lua/extensions.md#model-tools) cover arguments, progress, cancellation and completion. [schema limits](lua/tool-schemas.md) define supported validation rules.

`before_command` hooks can inspect or transform native command proposals before approval and again before launch. a changed execution tuple needs a new approval, and policy failure stops execution. the [scope example](lua/examples.md#native-command-scope) demonstrates an exact-command policy. these hooks do not constrain a trusted plugin's direct process or network access.
