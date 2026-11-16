package variables

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/syntax"
)

func (s Snapshot) Shell(command string) (string, error) {
	if !strings.Contains(command, "wraith::") {
		return command, nil
	}
	if _, err := s.Validate(command, true); err != nil {
		return "", err
	}
	tree, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(command), "")
	if err != nil {
		return "", fmt.Errorf("private command requires supported shell syntax: %w", err)
	}
	allowed := map[*syntax.Word]bool{}
	syntax.Walk(tree, func(node syntax.Node) bool {
		switch n := node.(type) {
		case *syntax.CallExpr:
			for _, word := range n.Args[min(1, len(n.Args)):] {
				allowed[word] = true
			}
		case *syntax.Assign:
			if n.Index == nil && n.Array == nil && n.Value != nil {
				allowed[n.Value] = true
			}
		case *syntax.Redirect:
			if n.Hdoc == nil && n.Op != syntax.Hdoc && n.Op != syntax.DashHdoc {
				allowed[n.Word] = true
			}
		}
		return true
	})
	type edit struct {
		start, end int
		text       string
	}
	var edits []edit
	syntax.Walk(tree, func(node syntax.Node) bool {
		if err != nil {
			return false
		}
		word, ok := node.(*syntax.Word)
		if !ok {
			return true
		}
		start, end := int(word.Pos().Offset()), int(word.End().Offset())
		if !strings.Contains(command[start:end], "wraith::") {
			return true
		}
		if !allowed[word] {
			err = errors.New("private references are allowed only in argument, assignment and redirect values")
			return false
		}
		for _, part := range word.Parts {
			if lit, ok := part.(*syntax.Lit); ok && strings.ContainsAny(lit.Value, "*?[~") {
				err = errors.New("quote literal wildcard and tilde text in private arguments")
				return false
			}
		}
		syntax.Walk(word, func(part syntax.Node) bool {
			if part == nil {
				return false
			}
			switch n := part.(type) {
			case *syntax.Word, *syntax.Lit:
			case *syntax.SglQuoted:
				if n.Dollar {
					err = errors.New("private references do not support dollar-quoted words")
				}
			case *syntax.DblQuoted:
				if n.Dollar {
					err = errors.New("private references do not support translated strings")
				}
			default:
				err = errors.New("private references cannot share a word with shell expansions")
			}
			return err == nil
		})
		if err != nil {
			return false
		}
		literal, decodeErr := expand.Literal(&expand.Config{}, word)
		if decodeErr != nil {
			err = decodeErr
			return false
		}
		if _, err = s.Validate(literal, true); err != nil {
			return false
		}
		resolved, _ := s.Expand(literal)
		quoted, quoteErr := syntax.Quote(resolved, syntax.LangPOSIX)
		if quoteErr != nil {
			err = quoteErr
			return false
		}
		edits = append(edits, edit{start, end, quoted})
		return false
	})
	if err != nil {
		return "", err
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	for _, edit := range edits {
		command = command[:edit.start] + edit.text + command[edit.end:]
	}
	if strings.Contains(command, "wraith::") {
		return "", errors.New("private reference appears outside a supported shell word")
	}
	return command, nil
}
