package rules

import (
	"fmt"
	"indicators/core"
	"math"
	"strconv"
	"strings"
	"time"
)

// describe: QuantMan Signal / Condition
//
// pipeConfigs:
// type: "signal"
// name: "signal"
// config:
//   signal:
//     - leftOperand: "timeOfDay"
//       operator: "isAbove"
//       rightOperand: "09:15:59"
//       conditionOperator: "and"
//
// Result:
// Current = 1 when conditions are true
// Current = 0 when conditions are false
// Previous = preceding ready signal
//
// Supported operands: timeOfDay
// Supported operators:
// equalTo, isAbove, isBelow,
// equalOrAbove, equalOrBelow
//
// Multiple conditions: AND / OR
//
// Note: QuantMan parity requires backtest verification.

type SignalRule struct {
	LeftOperand       string
	Operator          string
	RightOperand      string
	ConditionOperator string
}

type ConditionConfig struct {
	Signal []SignalRule
}

type Condition struct {
	id        string
	cfg       ConditionConfig
	lastTime  time.Time
	hasResult bool
	result    ScalarResult
}

func NewCondition(id string, cfg map[string]any) (*Condition, error) {
	if id == "" {
		return nil, fmt.Errorf("Signal: name cannot be empty")
	}

	raw, exists := cfg["signal"]
	if !exists {
		return nil, fmt.Errorf("Signal: signal config required")
	}

	items, ok := raw.([]any)
	if !ok || len(items) == 0 {
		return nil, fmt.Errorf("Signal: expected non-empty signal array")
	}

	rules := make([]SignalRule, 0, len(items))

	for index, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("Signal: invalid rule at index %d", index)
		}

		left, ok1 := m["leftOperand"].(string)
		operator, ok2 := m["operator"].(string)
		right, ok3 := m["rightOperand"].(string)

		if !ok1 || !ok2 || !ok3 {
			return nil, fmt.Errorf("Signal: invalid operands at index %d", index)
		}

		conditionOperator := "and"
		if value, exists := m["conditionOperator"]; exists {
			s, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("Signal: invalid conditionOperator at index %d", index)
			}
			conditionOperator = strings.ToLower(strings.TrimSpace(s))
		}

		if left != "timeOfDay" {
			return nil, fmt.Errorf("Signal: unsupported operand %q", left)
		}

		switch operator {
		case "equalTo", "isAbove", "isBelow", "equalOrAbove", "equalOrBelow":
		default:
			return nil, fmt.Errorf("Signal: unsupported operator %q", operator)
		}

		if conditionOperator != "and" && conditionOperator != "or" {
			return nil, fmt.Errorf("Signal: unsupported conditionOperator %q", conditionOperator)
		}

		if _, err := parseSignalTime(right); err != nil {
			return nil, fmt.Errorf("Signal: rule %d: %w", index, err)
		}

		rules = append(rules, SignalRule{
			LeftOperand:       left,
			Operator:          operator,
			RightOperand:      right,
			ConditionOperator: conditionOperator,
		})
	}

	return &Condition{
		id: id,
		cfg: ConditionConfig{
			Signal: rules,
		},
	}, nil
}

func (r *Condition) Update(c core.MarketCandle) any {
	r.ingest(c)
	return r.result
}

func (r *Condition) Result() any {
	return r.result
}

func (r *Condition) ingest(c core.MarketCandle) {
	if c == nil {
		return
	}

	t := c.GetTime()

	if t.IsZero() || (!r.lastTime.IsZero() && !t.After(r.lastTime)) {
		return
	}

	matched := false

	for i, rule := range r.cfg.Signal {
		value := evaluateSignalRule(rule, t)

		if i == 0 {
			matched = value
			continue
		}

		// The conditionOperator on each subsequent rule
		// connects that rule with the accumulated result.
		switch rule.ConditionOperator {
		case "and":
			matched = matched && value
		case "or":
			matched = matched || value
		}
	}

	current := 0.0
	if matched {
		current = 1
	}

	previous := current
	if r.hasResult {
		previous = r.result.Current
	}

	r.result = ScalarResult{
		IsReady:   true,
		Current:   current,
		Previous:  previous,
		Timestamp: t,
	}

	r.lastTime = t
	r.hasResult = true
}

func evaluateSignalRule(rule SignalRule, t time.Time) bool {
	threshold, err := parseSignalTime(rule.RightOperand)
	if err != nil {
		return false
	}

	current := t.Hour()*3600 + t.Minute()*60 + t.Second()

	switch rule.Operator {
	case "equalTo":
		return current == threshold
	case "isAbove":
		return current > threshold
	case "isBelow":
		return current < threshold
	case "equalOrAbove":
		return current >= threshold
	case "equalOrBelow":
		return current <= threshold
	default:
		return false
	}
}

func parseSignalTime(value string) (int, error) {
	parts := strings.Split(strings.TrimSpace(value), ":")

	if len(parts) != 2 && len(parts) != 3 {
		return 0, fmt.Errorf("invalid time %q", value)
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, err
	}

	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, err
	}

	second := 0

	if len(parts) == 3 {
		second, err = strconv.Atoi(parts[2])
		if err != nil {
			return 0, err
		}
	}

	if hour < 0 || hour > 23 ||
		minute < 0 || minute > 59 ||
		second < 0 || second > 59 {
		return 0, fmt.Errorf("invalid time %q", value)
	}

	return int(math.Round(float64(hour*3600 + minute*60 + second))), nil
}
