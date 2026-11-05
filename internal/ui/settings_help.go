package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

var settingDescriptions = map[string]string{
	"provider":                 "Choose Ollama, an API backend or an installed CLI. Selecting a provider opens its model catalog unless a default model is configured. The conversation remains available, and subsequent requests use the selected backend. API providers receive the context you send; credentials are configured per profile.",
	"providers":                "Named backend profiles in config.yaml. Configure kind, base_url, key_env or api_key, headers, model catalogs, cli command, thinking capabilities, extra request options and system_prompt. Environment credentials take precedence over inline API keys for HTTP providers. CLI profiles reuse the installed tool login unless explicit credentials are configured. Profile secrets are hidden from settings lists.",
	"execution.auto_share":     "Off by default. Share completed command output explicitly with the output_share binding (ctrl+g). When enabled, completed tracked commands share a bounded excerpt and continue the conversation automatically; terminal snapshots are included before the next model request. The manual share hint is hidden. Full tracked output stays in /history.",
	"clipboard":                "Executable and arguments used to copy selected text to the system clipboard. Default [wl-copy] targets Wayland; X11 users can use [xclip, -selection, clipboard]. An empty array disables system copying. Text is also stored in the Wraith prompt register, so normal-mode p still works when the external helper fails.",
	"model":                    "The Ollama model used for new chats. The picker lists installed models. Selecting one saves the default, keeps the current conversation, resets thinking to auto, and loads the model immediately. Wraith does not download missing weights.",
	"thinking":                 "Native model reasoning control. Auto leaves the model default intact. On/off are available for boolean thinking models; low/medium/high appear only when advertised by the model. Thinking consumes output tokens. This controls generation, not whether reasoning is visible.",
	"context":                  "Requested context capacity in tokens, as any positive integer. It includes instructions, pinned facts, conversation and the reserved output budget. 8192 is the default, not a limit. Presets are shortcuts; custom accepts values such as 1000000. Ollama and the model determine the usable capacity; requesting a larger window does not extend model training. Larger values use more memory and may cause CPU offload. Changing this setting reloads the model. Check ollama ps for actual GPU placement.",
	"predict":                  "Maximum generated tokens per response, including thinking. Any positive integer. The input and output reservation must fit before sending. Wraith reserves this budget before sending. A response that reaches the limit may be incomplete.",
	"threads":                  "CPU inference threads, from 0 to 512. Zero uses Ollama's default. This does not reserve GPU resources. Changing it reloads the model. Start with a modest count if other desktop work needs CPU time.",
	"auto_start":               "When enabled, start ollama serve if a plain HTTP loopback connection is refused. Existing servers are reused. Remote servers, permission errors and other connection failures do not trigger startup. Logs go to the cache directory as ollama.log. The shared server remains running after Wraith exits.",
	"execution.mode":           "Agent mode lets the model request one command when execution is useful. Every request still requires your explicit approval. Results stay private until you share them or enable execution.auto_share. Blocks mode only generates text and code; you can manually edit or run a block. Switch modes without changing models.",
	"execution.max_steps":      "Maximum approved model commands in one user turn, from 1 to 100. The default is 8. Each command requires approval. Once the limit is reached, send a new prompt to continue. Manual block runs are not part of an autonomous continuation.",
	"execution.output_context": "Maximum command output bytes considered for the model, from 256 to 65536. The actual excerpt is also capped to the context window. Full captured output, up to 8 MiB per run, remains in /history. Raising this value can leave less room for conversation.",
	"compact.mode":             "Ask prompts at the configured threshold. A decline postpones the prompt until the next qualifying response; the third decline compacts. Auto compacts without prompting. Manual disables threshold compaction. Oversized input still requires compaction, trimming or a larger context before it can be sent.",
	"compact.threshold":        "Estimated context fraction that triggers compaction, from 0.5 to 0.95. Default 0.9 means 90 percent. Output reservations can require compaction earlier. The estimate is conservative and calibrated against Ollama's measured prompt usage.",
	"compact.keep_turns":       "Recent user/assistant pairs retained verbatim during compaction, from 1 to 20. Default 2. Older turns are extracted and independently audited. More retained turns consume more active context and can prevent a useful reduction.",
	"compact.budget":           "Output-token budget for each extraction and audit request. At least 256. The extraction input and output must fit the context. Default 1536. Too small a budget can truncate extraction; truncated or invalid output never replaces the original chat. Immutable archives preserve the exact source.",
	"theme":                    "Select a built-in or custom named palette. Changes apply across the workspace immediately. Shells can inherit the host terminal colors or use this palette. Custom palettes live in the themes section of config.yaml.",
	"themes":                   "A map of custom palette names. Each requires bg, fg, muted, accent, second, border and warn as quoted #rrggbb strings. A custom name appears in /theme; using an existing name overrides its palette. Theme colors do not change terminal fonts or alpha opacity.",
	"appearance.background":    "Theme paints the app with its palette background. Inherit emits the host terminal's default background, allowing its configured transparency to show the wallpaper. Wraith cannot set an independent alpha percentage; change opacity and blur in your terminal app. Pane backgrounds have their own terminal.background setting.",
	"appearance.thinking":      "Default visibility of model reasoning. Each thinking section can also be expanded or collapsed with the thinking toggle key or a click. /thinking toggles all sections. Models must actually return thinking text for it to appear.",
	"appearance.mouse":         "Enable clicks on thinking headers. Disable to avoid app mouse capture. Your terminal may allow text selection while holding Shift. Mouse settings do not change normal-mode keyboard navigation.",
	"animations":               "Animate activity indicators at ten frames per second. Disabling reduces the idle redraw cadence to once per second. Responses and shell output still update when new data arrives.",
	"terminal.overlay_width":   "Centered scratch shell width as a fraction of terminal columns, from 0.3 to 0.95. Default 0.7. Existing panes resize immediately. Physical terminal dimensions still impose minimum sizes.",
	"terminal.overlay_height":  "Centered scratch shell height as a fraction of terminal rows, from 0.3 to 0.95. Default 0.6. Existing panes resize immediately, including the underlying PTY.",
	"terminal.split_width":     "Right shell width as a fraction of terminal columns, from 0.25 to 0.7. Default 0.4. The side shell fills the height below the header and above the status/shortcut bars. The prompt and conversation use the remaining width.",
	"terminal.background":      "Inherit uses the terminal default background and clears app content beneath the pane. Theme uses Wraith's palette. A quoted #rrggbb chooses an explicit color. Wallpaper visibility depends on the host terminal's opacity settings.",
	"terminal.foreground":      "Inherit uses the host terminal's default text color. Theme uses the app foreground. A quoted #rrggbb sets an explicit foreground. Applications inside the shell can still emit their own colors.",
	"terminal.scrollback":      "Retained scrollback lines per shell, from 0 to 100000. Default 5000. Shift+PageUp and Shift+PageDown scroll it. Changes apply to newly started panes; use /terminal-restart 1 or 2 to recreate a pane.",
	"terminal.directory":       "Initial directory for new shells and tracked command jobs. Empty uses the directory where Wraith launched. A ~/ prefix expands to your home. Existing interactive shells keep their current directory until restarted. Tracked jobs always start fresh here.",
	"shell":                    "Executable and arguments for interactive terminal panes, for example [zsh, -i]. Tracked jobs use the same executable with -c and do not share pane variables or directory changes. Shell changes apply to newly started panes. Use the interactive panes for programs requiring a TTY.",
	"leader":                   "Normal-mode prefix key for the command hint menu. Default space; any single character can be used. It is ignored in insert mode and shell input. Press the leader, then the displayed one-key or multi-key sequence. Escape closes the menu.",
	"bindings":                 "Map leader sequences to commands without a leading slash, for example b: blocks or 'c c': compact. Spaces separate sequence keys. An explicit map replaces the defaults. Prefix conflicts are rejected. Slash is reserved for searching the menu.",
	"keys":                     "Direct action bindings, including send, editor, focus, cancel, quit, pane toggles, pin_edit, pin_remove, thinking_toggle and setting_help. Global actions must have distinct keys. Pin and setting keys apply only in their picker; thinking_toggle applies in the transcript. Choose keys your terminal forwards.",
	"editor":                   "Executable and arguments for the external editor, default [nvim]. The file path is appended as a literal argument. Save and exit applies changes; quitting without saving discards them. Invalid saved edits remain on disk with a recovery path. This does not depend on the EDITOR environment variable.",
	"edit":                     "Open the full config.yaml in your configured editor. Save and exit to validate and apply it. Invalid settings do not replace the active configuration; the temporary file remains available for correction.",
}

func (a *App) settingHelp(k tea.KeyMsg) bool {
	m := a.popup
	if m.Searching || (m.Kind != "settings" && m.Kind != "value") || k.String() != a.Config.Keys["setting_help"] {
		return false
	}
	if m.Index < 0 || m.Index >= len(m.Items) {
		return true
	}
	key := m.Items[m.Index].Value
	key, _, _ = strings.Cut(key, "=")
	description := settingDescriptions[key]
	if strings.HasPrefix(key, "category.") {
		description = "Open this category to choose a setting. Press " + a.Config.Keys["setting_help"] + " on any setting to read its purpose, limits and behavior. Use j/k or arrows to move, Enter to select, and / to search."
	}
	if description == "" {
		description = "Edit this value in config.yaml. Changes are validated before being applied."
	}
	a.helpReturn = m
	a.showText("setting-help", key+" · esc returns", description+"\n\nUse / to search this explanation; j/k or arrows to scroll.")
	return true
}
