package price_reference

import (
	"fmt"
	"indicators/core"
	"math"
	"strconv"
	"strings"
	"time"
)

// describe: Entry Candle
//
// QuantMan:
// type: entryCandle
// name: entryCandle
//
// Config:
// candleInterval: "1 minutes"
// resetOnDayStart: true
// trailAnchor: false
// when: [{leftOperand:"timeOfDay", operator:"isAbove",
//         rightOperand:"09:15:59", conditionOperator:"and"}]
//
// Current = selected anchor candle.
// Previous = previously selected anchor candle.
// No OHLCV snapshot duplication.
//
// Note: Anchor behavior is an implementation interpretation.
// Exact QuantMan parity must be verified with backtest results.

type EntryCandleConfig struct {
	CandleInterval  string
	ResetOnDayStart bool
	TrailAnchor     bool
	When            []EntryCandleCondition
}

type EntryCandleCondition struct {
	LeftOperand       string
	Operator          string
	RightOperand      string
	ConditionOperator string
}

type EntryCandle struct {
	id  string
	cfg EntryCandleConfig

	lastTime   time.Time
	currentDay string

	anchor         core.MarketCandle
	previousAnchor core.MarketCandle

	result EntryCandleResult
}

func NewEntryCandle(id string, cfg map[string]any) (*EntryCandle, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("EntryCandle: empty name")
	}

	config := EntryCandleConfig{
		CandleInterval:  "1 minutes",
		ResetOnDayStart: false,
		TrailAnchor:     false,
	}

	if v, exists := cfg["candleInterval"]; exists {
		s, ok := v.(string)
		if !ok || strings.TrimSpace(s) == "" {
			return nil, fmt.Errorf("EntryCandle: invalid candleInterval")
		}
		config.CandleInterval = strings.TrimSpace(s)
	}

	if v, exists := cfg["resetOnDayStart"]; exists {
		b, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("EntryCandle: resetOnDayStart must be bool")
		}
		config.ResetOnDayStart = b
	}

	if v, exists := cfg["trailAnchor"]; exists {
		b, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("EntryCandle: trailAnchor must be bool")
		}
		config.TrailAnchor = b
	}

	rawWhen, exists := cfg["when"]
	if !exists {
		return nil, fmt.Errorf("EntryCandle: when condition required")
	}

	items, ok := rawWhen.([]any)
	if !ok || len(items) == 0 {
		return nil, fmt.Errorf("EntryCandle: when must be a non-empty array")
	}

	for i, item := range items {
		rule, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("EntryCandle: when[%d] must be an object", i)
		}

		left, _ := rule["leftOperand"].(string)
		op, _ := rule["operator"].(string)
		right, _ := rule["rightOperand"].(string)
		join, _ := rule["conditionOperator"].(string)

		if left != "timeOfDay" {
			return nil, fmt.Errorf("EntryCandle: unsupported operand %q", left)
		}

		switch op {
		case "isAbove", "isBelow", "equalTo", "equalOrAbove", "equalOrBelow":
		default:
			return nil, fmt.Errorf("EntryCandle: unsupported operator %q", op)
		}

		if _, err := parseEntryCandleTime(right); err != nil {
			return nil, fmt.Errorf("EntryCandle: when[%d]: %w", i, err)
		}

		if join == "" {
			join = "and"
		}

		if join != "and" && join != "or" {
			return nil, fmt.Errorf("EntryCandle: unsupported conditionOperator %q", join)
		}

		config.When = append(config.When, EntryCandleCondition{
			LeftOperand:       left,
			Operator:          op,
			RightOperand:      right,
			ConditionOperator: join,
		})
	}

	return &EntryCandle{
		id:  id,
		cfg: config,
	}, nil
}

func (r *EntryCandle) Update(c core.MarketCandle) any {
	r.ingest(c)
	return r.result
}

func (r *EntryCandle) Result() any {
	return r.result
}

func (r *EntryCandle) ingest(c core.MarketCandle) {
	if !validEntryCandle(c, r.lastTime) {
		return
	}

	t := c.GetTime()
	day := t.Format("2006-01-02")

	if r.currentDay != "" && day != r.currentDay && r.cfg.ResetOnDayStart {
		r.anchor = nil
		r.previousAnchor = nil
	}

	r.currentDay = day
	r.lastTime = t

	// Keep selected anchor until a qualifying candle arrives.
	if !r.matchesWhen(t) {
		r.updateResult(t)
		return
	}

	// trailAnchor=false: freeze the first matching candle.
	// trailAnchor=true: update anchor on each matching candle.
	if r.anchor == nil || r.cfg.TrailAnchor {
		if r.anchor != nil {
			r.previousAnchor = r.anchor
		}

		r.anchor = c
	}

	r.updateResult(t)
}

func (r *EntryCandle) updateResult(t time.Time) {
	if r.anchor == nil {
		r.result = EntryCandleResult{
			IsReady:   false,
			Timestamp: t,
		}
		return
	}

	previous := r.previousAnchor
	if previous == nil {
		previous = r.anchor
	}

	r.result = EntryCandleResult{
		IsReady:   true,
		Current:   r.anchor,
		Previous:  previous,
		Timestamp: t,
	}
}

func (r *EntryCandle) matchesWhen(t time.Time) bool {
	if len(r.cfg.When) == 0 {
		return false
	}

	now := t.Hour()*3600 + t.Minute()*60 + t.Second()

	var result bool

	for i, condition := range r.cfg.When {
		target, err := parseEntryCandleTime(condition.RightOperand)
		if err != nil {
			return false
		}

		matched := false

		switch condition.Operator {
		case "isAbove":
			matched = now > target
		case "isBelow":
			matched = now < target
		case "equalTo":
			matched = now == target
		case "equalOrAbove":
			matched = now >= target
		case "equalOrBelow":
			matched = now <= target
		}

		if i == 0 {
			result = matched
			continue
		}

		switch r.cfg.When[i-1].ConditionOperator {
		case "or":
			result = result || matched
		default:
			result = result && matched
		}
	}

	return result
}

func parseEntryCandleTime(value string) (int, error) {
	parts := strings.Split(strings.TrimSpace(value), ":")

	if len(parts) != 2 && len(parts) != 3 {
		return 0, fmt.Errorf("invalid time %q", value)
	}

	numbers := make([]int, len(parts))

	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil {
			return 0, fmt.Errorf("invalid time %q", value)
		}
		numbers[i] = n
	}

	hour := numbers[0]
	minute := numbers[1]
	second := 0

	if len(numbers) == 3 {
		second = numbers[2]
	}

	if hour < 0 || hour > 23 ||
		minute < 0 || minute > 59 ||
		second < 0 || second > 59 {
		return 0, fmt.Errorf("invalid time %q", value)
	}

	return hour*3600 + minute*60 + second, nil
}

func validEntryCandle(c core.MarketCandle, lastTime time.Time) bool {
	if c == nil {
		return false
	}

	t := c.GetTime()

	if t.IsZero() {
		return false
	}

	if !lastTime.IsZero() && !t.After(lastTime) {
		return false
	}

	o := c.GetOpen()
	h := c.GetHigh()
	l := c.GetLow()
	cl := c.GetClose()
	v := c.GetVolume()
	oi := c.GetOpenInterest()

	for _, x := range []float64{o, h, l, cl} {
		if math.IsNaN(x) || math.IsInf(x, 0) || x <= 0 {
			return false
		}
	}

	for _, x := range []float64{v, oi} {
		if math.IsNaN(x) || math.IsInf(x, 0) || x < 0 {
			return false
		}
	}

	if h < l || h < math.Max(o, cl) || l > math.Min(o, cl) {
		return false
	}

	return true
}
