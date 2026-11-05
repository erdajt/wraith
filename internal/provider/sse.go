package provider

import (
	"bufio"
	"errors"
	"io"
	"strings"
)

func readSSE(source io.Reader, handle func(string, []byte) error) error {
	scanner := bufio.NewScanner(source)
	scanner.Buffer(make([]byte, 64<<10), 4<<20)
	event := ""
	var data strings.Builder
	flush := func() error {
		if data.Len() == 0 {
			return nil
		}
		err := handle(event, []byte(strings.TrimSuffix(data.String(), "\n")))
		event = ""
		data.Reset()
		return err
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := flush(); err != nil {
				return err
			}
			continue
		}
		field, value, _ := strings.Cut(line, ":")
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "event":
			event = value
		case "data":
			data.WriteString(value)
			data.WriteByte('\n')
		}
		if data.Len() > 4<<20 {
			return errors.New("provider event exceeds 4 mib")
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return flush()
}
