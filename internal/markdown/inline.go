package markdown

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/erdajt/wraith/internal/config"
)

const reset = "\x1b[0m"

var link = regexp.MustCompile(`^!?\[([^\]]*)\]\(([^\s)]+)(?:\s+"[^"]*")?\)`)

func rgb(hex string) string {
	if len(hex) != 7 {
		return ""
	}
	value, err := strconv.ParseUint(hex[1:], 16, 32)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("38;2;%d;%d;%d", value>>16, (value>>8)&255, value&255)
}

func paint(color, text string) string { return "\x1b[" + rgb(color) + "m" + text + reset }

func inline(text string, p config.Palette, flags string, depth int) string {
	style := "\x1b[" + rgb(p.FG) + flags + "m"
	if depth > 8 {
		return style + text + reset
	}
	var out strings.Builder
	out.WriteString(style)
	for i := 0; i < len(text); {
		if text[i] == '\\' && i+1 < len(text) && strings.ContainsRune(`\`+"`*_{}[]()#+-.!>|~", rune(text[i+1])) {
			out.WriteByte(text[i+1])
			i += 2
			continue
		}
		if text[i] == '`' {
			n := 1
			for i+n < len(text) && text[i+n] == '`' {
				n++
			}
			marker := strings.Repeat("`", n)
			if end := strings.Index(text[i+n:], marker); end >= 0 {
				out.WriteString(paint(p.Accent, text[i+n:i+n+end]))
				out.WriteString(style)
				i += 2*n + end
				continue
			}
		}
		if text[i] == '[' || text[i] == '!' {
			if m := link.FindStringSubmatch(text[i:]); m != nil {
				out.WriteString(inline(m[1], p, flags+";4", depth+1))
				out.WriteString(paint(p.Muted, " ("+m[2]+")"))
				out.WriteString(style)
				i += len(m[0])
				continue
			}
		}
		matched := false
		for _, d := range []struct{ mark, flag string }{{"***", ";1;3"}, {"___", ";1;3"}, {"**", ";1"}, {"__", ";1"}, {"~~", ";9"}, {"*", ";3"}, {"_", ";3"}} {
			if !strings.HasPrefix(text[i:], d.mark) {
				continue
			}
			if d.mark[0] == '_' && i > 0 && word(text[i-1]) {
				continue
			}
			rest := text[i+len(d.mark):]
			end := strings.Index(rest, d.mark)
			if end >= 0 && len(d.mark) == 2 && d.mark != "~~" && strings.HasPrefix(rest[end:], d.mark+d.mark[:1]) && strings.Count(rest[:end], d.mark[:1])%2 == 1 {
				end++
			}
			if end <= 0 || rest[0] == ' ' || rest[end-1] == ' ' {
				continue
			}
			out.WriteString(inline(rest[:end], p, flags+d.flag, depth+1))
			out.WriteString(style)
			i += 2*len(d.mark) + end
			matched = true
			break
		}
		if matched {
			continue
		}
		_, n := utf8.DecodeRuneInString(text[i:])
		out.WriteString(text[i : i+n])
		i += n
	}
	out.WriteString(reset)
	return out.String()
}

func word(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_'
}
