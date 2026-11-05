package terminal

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func Key(k tea.KeyMsg, application, bracketed bool) []byte {
	key := k.String()
	prefix := "\x1b["
	if application {
		prefix = "\x1bO"
	}
	known := map[string]string{"up": prefix + "A", "down": prefix + "B", "right": prefix + "C", "left": prefix + "D", "home": "\x1b[H", "end": "\x1b[F", "pgup": "\x1b[5~", "pgdown": "\x1b[6~", "delete": "\x1b[3~", "insert": "\x1b[2~", "enter": "\r", "backspace": "\x7f", "tab": "\t", "shift+tab": "\x1b[Z", "esc": "\x1b", "f1": "\x1bOP", "f2": "\x1bOQ", "f3": "\x1bOR", "f4": "\x1bOS", "f5": "\x1b[15~", "f6": "\x1b[17~", "f7": "\x1b[18~", "f8": "\x1b[19~", "f9": "\x1b[20~", "f10": "\x1b[21~", "f11": "\x1b[23~", "f12": "\x1b[24~"}
	if v, ok := known[key]; ok {
		return []byte(v)
	}
	if strings.HasPrefix(key, "ctrl+") {
		r := strings.TrimPrefix(key, "ctrl+")
		if len(r) == 1 {
			return []byte{r[0] & 0x1f}
		}
	}
	if k.Type == tea.KeyRunes {
		text := string(k.Runes)
		if k.Paste && bracketed {
			return []byte("\x1b[200~" + text + "\x1b[201~")
		}
		if k.Alt {
			return []byte("\x1b" + text)
		}
		return []byte(text)
	}
	return nil
}
