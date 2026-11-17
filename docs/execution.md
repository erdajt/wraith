# commands and evidence

`execution.mode` selects `agent` or `blocks`. both distinguish concrete execution requests from ordinary markdown examples. neither runs a command merely because it appeared in a response.

in agent mode, approved commands return their input, status and bounded output to the model, which can continue the task. blocks mode keeps the approval flow but shares output only when requested, unless `execution.auto_share` is enabled. an explicit hide-output choice overrides either mode.

## approval

execution requests display the resolved command and its `reason:`. transport markers and json fields are decoded before chat rendering, selection, copying or editor handoff. unresolved placeholders remain suggestions. read the full command before approving; placeholder detection cannot recognize every possible template.

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

saving a shell block stages its actual text in the scratch shell. enter inserts it into shell input without a newline; the next enter executes it. multiline insertion requires bracketed paste support. `r` offers a tracked run, `e` edits again, and escape dismisses staging. source languages such as python and go use a saved file and interpreter. private references in non-shell source require editor review before running; Wraith does not infer language-specific quoting.

## background jobs and history

tracked jobs use the configured shell executable with `-c` in `terminal.directory`, or the launch directory when empty. each job starts a fresh shell with the active private bindings. use a scratch pane for persistent interactive state or programs needing a tty.

an agent may request a background job, labelled in approval. after approval it receives a start receipt and can keep working. `/jobs` or `<leader>j` shows current-session jobs; `/history` shows saved command records. open a job to view live output, status and exit code. `ctrl+x` cancels the selected job, configurable as `keys.job_cancel`.

output readers support vim motions, counts, `/` search, visual selection and yanks. selection freezes the visible snapshot so incoming output does not move it. `r` refreshes it. live views show the recent tail; completed views include the retained prefix and tail with gap offsets.

completion evidence waits while you edit, type a draft, select text, use a shell or review a popup. it joins the next safe request without replacing your draft. session delivery ids prevent duplicate insertion after resume. jobs from an older turn do not independently restart that turn.

## output access

agent-mode tracked runs share by default. blocks-mode runs share only through `ctrl+g` in the output viewer or `execution.auto_share: true`. explicitly hidden output remains private until you choose to share it. the manual binding appears only when sharing is needed. sharing a record twice does not duplicate evidence; cancelled runs can be shared without automatically continuing.

completion evidence uses a bounded excerpt, reserving space for the retained tail when capture overflowed. the model can poll new lines or search authorized output by literal text or regular expression, then read bounded pages with source line numbers. tools only access shared jobs in the current session. matching occurs after private-value redaction. withheld jobs cannot be queried.

read pages contain at most 200 lines and 16 kib before the smaller configured context budget applies. queries allow at most ten context lines before/after a match and a 4096-byte pattern. `ignore_case` supports case-insensitive literal or regex matching. `from` and the returned `next` cursor are one-based output line numbers; omitted or zero `from` starts at the first line. omitted or zero `limit` uses 80 lines. byte ranges describe capture gaps only. pages include observed/prefix/tail bytes, retained/matched line counts, elapsed milliseconds and omitted byte ranges. oversized lines are marked omitted so polling still advances. when context cannot hold more evidence, reads wait for compaction without advancing their cursor or consuming their budget. polls are limited to one per second per job; their cursor advances only after the result is saved. full retained output remains available locally.

`execution.max_steps` limits approved model commands per user turn; `execution.max_reads` limits output queries. reaching either pauses continuation. a new prompt resets these budgets. all command output is untrusted evidence, not new instructions.

in scratch copy mode, `ctrl+g` shares the selection or snapshot. `execution.auto_share` includes changed snapshots before the next request. snapshots cannot reliably identify command boundaries, exit status or discarded scrollback; use tracked jobs when those matter.

## limits and recovery

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

if the model sends an output query through the command channel, Wraith rejects it without executing anything and supplies a specific correction on retry. the same applies to a command sent as an output query. internal block names appear only in model-facing feedback, while the status explains the mismatch in plain language. the existing three-retry limit still applies.
