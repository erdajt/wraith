package ui

import ()

func helpItems() []item {
	entries := [][2]string{{"/models · /model", "choose an installed ollama model"}, {"/think [auto|off|on|low|medium|high]", "only native supported values are accepted"}, {"/thinking", "toggle reasoning display"}, {"/settings · /set key value", "edit defaults, bindings and terminal settings"}, {"/theme [name]", "phosphor, tokyo, ember, ice, mono"}, {"/context", "estimated usage and last measured prompt"}, {"/compact", "archive, extract, audit and compact"}, {"/instructions · /memory", "edit persistent user instructions"}, {"/remember text", "append an instruction for every chat"}, {"/pin text · /pins · /unpin number", "preserve exact facts in this chat"}, {"/blocks", "choose a fenced block to edit and stage"}, {"/edit · /edit-response", "edit the prompt or selected response"}, {"/new · /chats · /resume id", "create and switch saved conversations"}, {"/archives · /restore name", "fork an archived original into a new chat"}, {"/export [path]", "export active context as markdown"}, {"/retry · /cancel", "retry an interrupted request or cancel work"}, {"/terminal-restart 1|2", "restart the overlay or split shell"}, {"ctrl+s · ctrl+e · tab", "send, external editor, switch focus"}, {"alt+e · alt+s · ctrl+\\", "overlay, right split, return focus to chat"}, {"chat: j k gg G ctrl+u ctrl+d / n N", "scroll and search responses"}, {"chat: [ ] e b", "select messages, edit response, open blocks"}, {"prompt: esc i a o v V d c y p u", "modal editing; ctrl+e opens full neovim"}, {"ctrl+q · /quit", "save and exit; closes child shells"}}
	items := make([]item, 0, len(entries))
	for _, e := range entries {
		items = append(items, item{e[0], "", e[1]})
	}
	return items
}
