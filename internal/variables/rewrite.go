package variables

import (
	"fmt"
	"strconv"
	"strings"
)

func Rewrite(text, name, value string, remove bool) (string, error) {
	if !namePattern.MatchString(name) {
		return "", fmt.Errorf("invalid variable name")
	}
	entries, err := Parse(text)
	if err != nil {
		return "", err
	}
	if !remove {
		if _, err := Parse(name + "=" + strconv.Quote(value)); err != nil {
			return "", err
		}
	}
	found := false
	for _, entry := range entries {
		if entry.Name == name {
			found = true
		}
	}
	if !found {
		if remove {
			return "", fmt.Errorf("variable %s is not defined in this scope", name)
		}
		if text != "" && !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		return text + name + "=" + strconv.Quote(value) + "\n", nil
	}
	lines := strings.Split(text, "\n")
	block := false
	for row, line := range lines {
		var quote byte
		codeStart, valueStart, valueEnd := -1, -1, len(line)
		for i := 0; i < len(line); i++ {
			if block {
				if strings.HasPrefix(line[i:], "*/") {
					block = false
					i++
				}
				continue
			}
			ch := line[i]
			if quote != 0 {
				if ch == '\\' && quote == '"' {
					i++
				} else if ch == quote {
					quote = 0
				}
				continue
			}
			separated := i == 0 || line[i-1] == ' ' || line[i-1] == '\t'
			if separated && (ch == '#' || strings.HasPrefix(line[i:], "//")) {
				valueEnd = i
				break
			}
			if separated && strings.HasPrefix(line[i:], "/*") {
				if valueStart >= 0 {
					valueEnd = min(valueEnd, i)
				}
				block = true
				i++
				continue
			}
			if codeStart < 0 && ch != ' ' && ch != '\t' {
				codeStart = i
			}
			if ch == '=' && valueStart < 0 {
				valueStart = i + 1
			}
			if ch == '\'' || ch == '"' {
				quote = ch
			}
		}
		if codeStart < 0 || valueStart < 0 || strings.TrimSpace(line[codeStart:valueStart-1]) != name {
			continue
		}
		suffix := line[valueEnd:]
		if remove {
			lines[row] = line[:codeStart] + suffix
			last := row
			if block {
				for last++; last < len(lines); last++ {
					if at := strings.Index(lines[last], "*/"); at >= 0 {
						if strings.TrimSpace(lines[last][at+2:]) != "" {
							return "", fmt.Errorf("remove this annotated assignment in the editor")
						}
						break
					}
				}
			}
			if last < len(lines) && strings.TrimSpace(lines[last]) != "" {
				lines[last] += "\n"
			}
		} else {
			lines[row] = line[:valueStart] + strconv.Quote(value)
			if suffix != "" {
				lines[row] += " " + suffix
			}
		}
		result := strings.Join(lines, "\n")
		if _, err := Parse(result); err != nil {
			return "", err
		}
		return result, nil
	}
	return "", fmt.Errorf("variable assignment could not be edited")
}
