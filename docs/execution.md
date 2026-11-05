# commands and evidence

`execution.mode` selects `agent` or `blocks`. agent mode lets the model request one concrete command when execution would help. blocks mode keeps suggestions as ordinary markdown. neither mode runs a command merely because it appeared in a response.

## approval

an execution request displays the command and its `reason:`. the transport marker and json fields are decoded before chat rendering, block selection, copying, exporting or editor handoff. incomplete or malformed requests cannot run. requests with unresolved placeholders stay suggestions.

choose an action with `j/k` or arrows and enter:

- **run** executes the displayed command and captures its output. selecting run is the approval; no second approval popup follows.
- **edit** opens the plain command in your configured editor. save and exit to review the changed command before running it.
- **view full command** opens a scrollable preview, including the reason.
- **ask assistant to modify** opens an inline revision field. enter sends the revision request. submitting an empty field prepares the main prompt with the command, reason and `modify:`; nothing is sent until you finish that prompt.
- **reject** records that the command was not executed.

a model request containing a template such as `<TARGET_IP_OR_HOST>` does not open approval. the model is instructed to ask for the missing value and use an ordinary shell block for examples. placeholder detection covers common forms, not every possible way to express a template; read commands before approving them.

## ordinary blocks

`<leader>b` or `/blocks` lists code blocks. enter on a command under the conversation cursor opens it directly in your editor. enter on an assistant message offers its blocks. each block has edit, run and view actions; run executes once and records the result.

saving a shell block stages its actual command text in the scratch shell. the first enter inserts it into shell input without a terminating newline; the next enter executes the shell input. multiline insertion requires bracketed paste support. `r` instead offers a tracked run, `e` edits again, and escape dismisses staging. source languages such as python and go use a saved file with their interpreter because they are not shell command text.

## output and history

tracked runs use the configured shell executable with `-c` in `terminal.directory`, or the launch directory when that setting is empty. each run starts a fresh shell. use a scratch pane when persistent variables or an interactive terminal are required.

`/history` opens compressed command records with timestamps, working directory, status, exit code and merged stdout/stderr. output capture is bounded at 8 mib per run and truncation is marked. the viewer supports vim motions and counts, `/` search, visual selection, `y`, `Y`, `ye` and `yf<char>`. `r` refreshes a frozen snapshot after more output arrives. clipboard copies also populate the prompt register.

output is **not shared with the model by default**. press `ctrl+g` in the output viewer to share it. this binding is configurable as `keys.output_share`. sharing adds a bounded excerpt to the conversation and continues the model; sharing the same record twice in a session does not duplicate it. cancelled runs can be shared without automatically continuing.

set `execution.auto_share: true` to share completed tracked runs automatically. the manual share hint disappears. excerpts are limited by `execution.output_context` and the current context budget; full captured output stays in history.

in a scratch terminal's triple-escape copy mode, the same share binding sends the visual selection or the snapshot. automatic sharing includes changed terminal snapshots before the next model request. terminal snapshots do not reliably identify command boundaries, exit status or text already lost from bounded scrollback, so they are labelled accordingly. use tracked runs for complete command records.

`execution.max_steps` limits model-requested runs in a user turn. every step still needs approval. shared output is treated as untrusted evidence. the model cannot treat instructions printed by a command as new user instructions.
