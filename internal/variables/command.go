package variables

import (
	"fmt"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

func (s Snapshot) CheckCommand(command string) error {
	if len(s.values) == 0 {
		return nil
	}
	tree, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(command), "")
	if err != nil {
		return nil
	}
	syntax.Walk(tree, func(node syntax.Node) bool {
		if err != nil {
			return false
		}
		word, ok := node.(*syntax.Word)
		if !ok || len(word.Parts) != 1 {
			return true
		}
		var literal string
		switch part := word.Parts[0].(type) {
		case *syntax.Lit:
			literal = part.Value
		case *syntax.SglQuoted:
			literal = part.Value
		case *syntax.DblQuoted:
			if len(part.Parts) == 1 {
				if text, ok := part.Parts[0].(*syntax.Lit); ok {
					literal = text.Value
				}
			}
		}
		if _, known := s.values[literal]; known {
			err = fmt.Errorf("bare private variable %s in command; use wraith::%s for its configured value", literal, literal)
		}
		return true
	})
	return err
}
