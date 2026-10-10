package condition

import (
	"indicators/pipline/momentum"
	"strings"
)

type Operator string

const (
	EqualTo      Operator = "Equal To"
	IsAbove      Operator = "Is Above"
	IsBelow      Operator = "Is Below"
	CrossesAbove Operator = "Crosses Above"
	CrossesBelow Operator = "Crosses Below"
	EqualOrAbove Operator = "Equal Or Above"
	EqualOrBelow Operator = "Equal Or Below"
)

// Rule defines an entry or exit condition rule.
// Indicator is optional; if omitted, the Strategy-level indicator is used.
type Rule struct {
	Indicator string   // Optional: defaults to Strategy.Indicator if empty
	Operator  Operator // EqualTo, IsAbove, IsBelow, CrossesAbove, etc.
	Value     float64  // Comparison threshold
}

// Condition is an alias for Rule for backward compatibility.
type Condition = Rule

// Evaluate checks if the condition is satisfied given current and previous indicator values.
func (r Rule) Evaluate(current, previous float64) bool {
	op := strings.ToLower(strings.TrimSpace(string(r.Operator)))

	switch op {
	case "equal to", "==":
		return current == r.Value

	case "is above", ">":
		return current > r.Value

	case "is below", "<":
		return current < r.Value

	case "crosses above":
		return previous <= r.Value && current > r.Value

	case "crosses below":
		return previous >= r.Value && current < r.Value

	case "equal or above", ">=":
		return current >= r.Value

	case "equal or below", "<=":
		return current <= r.Value

	default:
		return false
	}
}

// EvaluateEntry checks entry conditions at bar close, handling new day open bars and standard crossovers.
func (r Rule) EvaluateEntry(isFirstBarOfDay, isReady bool, current, previous float64) bool {
	if !isReady {
		return false
	}

	if isFirstBarOfDay {
		op := strings.ToLower(strings.TrimSpace(string(r.Operator)))
		switch op {
		case "crosses above":
			return current > r.Value
		case "crosses below":
			return current < r.Value
		}
	}

	return r.Evaluate(current, previous)
}

// CanEnterAtMarketOpen determines if a level condition triggers an entry at market open (09:15).
func (r Rule) CanEnterAtMarketOpen(hasConfirmed bool, lastValue float64) bool {
	op := strings.ToLower(strings.TrimSpace(string(r.Operator)))

	// Crossovers require two closed candles, so they never trigger at market open
	if strings.Contains(op, "cross") {
		return false
	}

	// Strict rule: If indicator has no confirmed value, never enter at market open
	if !hasConfirmed {
		return false
	}

	switch op {
	case "equal to", "==":
		return lastValue == r.Value

	case "is above", ">":
		return lastValue > r.Value

	case "is below", "<":
		return lastValue < r.Value

	case "equal or above", ">=":
		return lastValue >= r.Value

	case "equal or below", "<=":
		return lastValue <= r.Value

	default:
		return false
	}
}

// ExtractValues dynamically extracts IsReady, Current, and Previous from any pipeline indicator result.
func ExtractValues(res any) (ready bool, current float64, previous float64, ok bool) {
	switch v := res.(type) {
	case momentum.RSIResult:
		return v.IsReady, v.Current, v.Previous, true
	case momentum.CCIResult:
		return v.IsReady, v.Current, v.Previous, true
	case momentum.AOResult:
		return v.IsReady, v.Current, v.Previous, true
	case momentum.MFIResult:
		return v.IsReady, v.Current, v.Previous, true
	case momentum.WilliamsPercentRangeResult:
		return v.IsReady, v.Current, v.Previous, true
	case momentum.ROCResult:
		return v.IsReady, v.Current, v.Previous, true
	default:
		return false, 0, 0, false
	}
}
