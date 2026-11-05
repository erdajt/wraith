# design

wraith is a local, keyboard-driven cybersecurity workspace written in go. the default model is `hf.co/filvyb/Qwen3.5-9B-heretic-v2-GGUF:Q5_K_M` through ollama's native chat api.

## requirements

- modal prompt editing, vim transcript navigation, response editing, and configurable neovim handoff.
- two persistent shell sessions: `alt+e` opens a centered overlay; `alt+s` opens a right split.
- saving a code block stages it for review. quitting without saving leaves it alone. staging does not execute it.
- terminal panes support inherited terminal backgrounds and colors. host terminal settings own font and opacity. inherited backgrounds clear covered app cells.
- themes, configurable bindings, shell, editor, native model thinking, context size, and output budget.
- persistent user instructions across chats and exact pins within a chat.
- compressed active sessions, separate compressed immutable archives, and a compaction recovery cache.
- bounded extraction and an independent omission audit for each transcript fragment. previous facts, pins, and recent turns survive compaction.
- automatic, prompted, and manual compaction. the default threshold is 90 percent. prompted mode allows three declines, then compacts.
- no silent context truncation. failed, empty, oversized, or truncated compaction leaves the active session unchanged.
- incremental conventional commits, focused tests, push/pr ci, tagged release builds, and complete usage documentation.

## boundaries

model summaries cannot guarantee preservation of every semantic detail. immutable source archives provide byte-for-byte recovery; pins provide exact active-context retention. the app must expose this distinction.

thinking controls use the capabilities reported by ollama. a model that supports only boolean thinking must not be presented as supporting native low/medium/high effort.

terminal output and model output are separate paths. model text is sanitized before display. only explicit user actions stage or execute commands. shell panes use real pseudoterminals and retain their working directory and process state while hidden.
