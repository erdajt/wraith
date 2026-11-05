package vim

func (b *Buffer) Key(key, text string) {
	if key == "esc" {
		if b.Mode == "insert" && b.Pos > b.lineStart(b.Pos) {
			b.Pos--
		}
		b.Mode = "normal"
		b.Pending = ""
		b.Count = ""
		b.clamp()
		return
	}
	if b.Mode == "search" {
		switch key {
		case "enter":
			b.Search = b.query
			b.Mode = "normal"
			b.find(false)
		case "backspace":
			r := []rune(b.query)
			if len(r) > 0 {
				b.query = string(r[:len(r)-1])
			}
		default:
			b.query += text
		}
		return
	}
	if b.Mode == "insert" {
		switch key {
		case "backspace":
			if b.Pos > 0 {
				b.remove(b.Pos-1, b.Pos)
			}
		case "delete":
			if b.Pos < len(b.Text) {
				b.remove(b.Pos, b.Pos+1)
			}
		case "enter":
			b.Insert("\n")
		case "tab":
			b.Insert("    ")
		case "left":
			b.Pos = max(0, b.Pos-1)
		case "right":
			b.Pos = min(len(b.Text), b.Pos+1)
		case "up":
			b.moveVertical(-1)
		case "down":
			b.moveVertical(1)
		case "home":
			b.Pos = b.lineStart(b.Pos)
		case "end":
			b.Pos = b.lineEnd(b.Pos)
		default:
			if text != "" {
				b.Insert(text)
			}
		}
		return
	}
	if key == "i" {
		b.Mode = "insert"
		return
	}
	if key == "u" {
		b.restore(false)
		return
	}
	if key == "ctrl+r" {
		b.restore(true)
		return
	}
	b.motion(key, 1)
}
