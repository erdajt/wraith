package runner

import (
	"context"
	"errors"
	"math"
	"strings"
)

type outputCursor struct {
	text                string
	line                int64
	skipFirst, complete bool
}

// next reads complete retained lines without allocating an index for the whole capture.
func (c *outputCursor) next(ctx context.Context) (OutputLine, bool, error) {
	for c.text != "" {
		if err := ctx.Err(); err != nil {
			return OutputLine{}, false, err
		}
		if c.line < 1 || c.line > math.MaxInt64-11 {
			return OutputLine{}, false, errors.New("invalid output line range")
		}
		text, rest, terminated := strings.Cut(c.text, "\n")
		c.text = rest
		number := c.line
		c.line++
		if !terminated && !c.complete {
			return OutputLine{}, false, nil
		}
		if c.skipFirst {
			c.skipFirst = false
			continue
		}
		return OutputLine{Number: number, Text: text}, true, nil
	}
	return OutputLine{}, false, nil
}
