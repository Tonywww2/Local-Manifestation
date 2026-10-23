package damage

import (
	"fmt"
	"math"
	"strconv"
)

// FormulaToken refers to the exact evaluator step responsible for an operand.
// Punctuation has Step -1; consumers localize explanations without recomputing.
type FormulaToken struct {
	Text string
	Step int
}

// Formula preserves evaluator order, including final additions, armor, floors,
// immunity and per-segment rounding. It never changes the evaluated result.
func Formula(r Result) ([]FormulaToken, error) {
	var out []FormulaToken
	precedence := 3
	literal := func(text string) FormulaToken { return FormulaToken{Text: text, Step: -1} }
	wrap := func() { out = append([]FormulaToken{literal("(")}, out...); out = append(out, literal(")")) }
	for i, s := range r.Steps {
		if math.IsNaN(s.Operand) || math.IsInf(s.Operand, 0) {
			return nil, fmt.Errorf("invalid formula operand")
		}
		value := formulaNumber(s.Operand)
		if s.Stat != "" {
			value = formulaNumber(s.Score) + " × " + formulaNumber(s.Weight)
		}
		if s.Operand < 0 {
			value = "(" + value + ")"
		}
		token := FormulaToken{Text: value, Step: i}
		if len(out) == 0 {
			if s.Operation != "add" && s.Operation != "base" && s.Operation != "override" {
				return nil, fmt.Errorf("formula has no base")
			}
			out = append(out, token)
			continue
		}
		switch s.Operation {
		case "add":
			out = append(out, literal(" + "), token)
			precedence = 1
		case "subtract":
			out = append(out, literal(" − "), token)
			precedence = 1
		case "multiply":
			if precedence < 2 {
				wrap()
			}
			out = append(out, literal(" × "), token)
			precedence = 2
		case "minimum":
			out = append([]FormulaToken{literal("max("), token, literal(", ")}, out...)
			out = append(out, literal(")"))
			precedence = 3
		case "override":
			if s.Operand != 0 {
				return nil, fmt.Errorf("unsupported formula override")
			}
			wrap()
			out = append(out, literal(" × "), token)
			precedence = 2
		default:
			return nil, fmt.Errorf("unknown formula operation %s", s.Operation)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty damage formula")
	}
	out = append([]FormulaToken{literal("round(")}, out...)
	out = append(out, literal(", 2) = "), literal(fmt.Sprintf("%.2f", r.Total)))
	return out, nil
}

func formulaNumber(value float64) string {
	if value == Round(value) {
		return fmt.Sprintf("%.2f", value)
	}
	return strconv.FormatFloat(value, 'g', -1, 64)
}
