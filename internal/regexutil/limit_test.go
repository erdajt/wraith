package regexutil

import (
	"regexp/syntax"
	"strings"
	"testing"
)

func TestRegexExpansionEstimateBoundsCompiledPrograms(t *testing.T) {
	for _, pattern := range []string{"", "abc", "(a|b)*", "(a?)*", "(a?){0,}", "(a?){4,}", "(a|bc){2,5}", "(?i)été", "[a-z]+", "a{0}", "(?:a?b?){3,8}"} {
		t.Run(pattern, func(t *testing.T) {
			expression, err := syntax.Parse(pattern, syntax.Perl)
			if err != nil {
				t.Fatal(err)
			}
			estimate := expandedCost(expression) + 2
			program, err := syntax.Compile(expression.Simplify())
			if err != nil {
				t.Fatal(err)
			}
			if estimate < len(program.Inst) {
				t.Fatalf("estimate %d below actual %d", estimate, len(program.Inst))
			}
		})
	}
}

func TestSizeRejectsUnboundedExpansion(t *testing.T) {
	for _, pattern := range []string{"[", strings.Repeat("x", 4097), "(" + strings.Repeat("x", 900) + "){1000}"} {
		if _, err := Size(pattern); err == nil {
			t.Fatal("invalid or oversized pattern accepted")
		}
	}
	if size, err := Size("a+b"); err != nil || size < 3 {
		t.Fatalf("valid expression: %d %v", size, err)
	}
}
