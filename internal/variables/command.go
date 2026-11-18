package variables

import (
	"fmt"
	"path/filepath"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// CheckCommand flags bare configured variable names in shell words, except recognized name operands.
func (s Snapshot) CheckCommand(command string) error {
	if len(s.values) == 0 {
		return nil
	}

	tree, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(command), "")
	if err != nil {
		return nil
	}

	names := map[*syntax.Word]bool{}
	syntax.Walk(tree, func(node syntax.Node) bool {
		if err != nil {
			return false
		}

		if call, ok := node.(*syntax.CallExpr); ok && len(call.Args) > 0 {
			args := call.Args
			if (args[0].Lit() == "command" || args[0].Lit() == "builtin") && len(args) > 1 {
				args = args[1:]
			}
			switch filepath.Base(args[0].Lit()) {
			case "unset", "printenv":
				for _, argument := range args[1:] {
					names[argument] = true
				}
			case "env":
				for i := 1; i < len(args); i++ {
					option := args[i].Lit()
					if option == "--" || !strings.HasPrefix(option, "-") && !strings.Contains(option, "=") {
						break
					}
					switch option {
					case "-u", "--unset":
						i++
						if i < len(args) {
							names[args[i]] = true
						}
					case "-C", "--chdir", "-S", "--split-string":
						i++
					}
				}
			}
		}

		word, ok := node.(*syntax.Word)
		if !ok || names[word] || len(word.Parts) != 1 {
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
