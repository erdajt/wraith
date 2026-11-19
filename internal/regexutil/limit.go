package regexutil

import (
	"errors"
	"regexp/syntax"
)

// Size validates bounded source and expansion before returning the compiled instruction count.
func Size(pattern string) (int, error) {
	if len(pattern) > 4096 {
		return 0, errors.New("regular expression exceeds source limit")
	}
	expression, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return 0, errors.New("invalid regular expression")
	}
	if expandedCost(expression)+2 > 4096 {
		return 0, errors.New("regular expression exceeds expanded instruction limit")
	}
	program, err := syntax.Compile(expression.Simplify())
	if err != nil {
		return 0, errors.New("invalid regular expression")
	}
	if len(program.Inst) > 4096 {
		return 0, errors.New("regular expression exceeds expanded instruction limit")
	}
	return len(program.Inst), nil
}

// expandedCost bounds expanded instructions before counted repetitions allocate copies.
func expandedCost(expression *syntax.Regexp) int {
	const overflow = 4097
	cost := 0
	for _, child := range expression.Sub {
		cost += expandedCost(child)
		if cost >= overflow {
			return overflow
		}
	}
	switch expression.Op {
	case syntax.OpLiteral:
		return len(expression.Rune)
	case syntax.OpConcat:
		return max(1, cost)
	case syntax.OpAlternate:
		cost += len(expression.Sub) - 1
	case syntax.OpCapture:
		cost += 2
	case syntax.OpStar:
		cost += 2
	case syntax.OpPlus, syntax.OpQuest:
		cost++
	case syntax.OpRepeat:
		if expression.Max < 0 {
			cost = cost*max(1, expression.Min) + 2
		} else if expression.Max == 0 {
			cost = 1
		} else {
			cost = cost*expression.Max + expression.Max - expression.Min
		}
	default:
		return 1
	}
	return min(cost, overflow)
}
