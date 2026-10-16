package price_volume

import (
	"fmt"
	"indicators/core"
	"math"
	"strconv"
	"strings"
	"time"
)

// Indicator: Anchored Volume Weighted Average Price
//
// QuantMan Pipe Config:
//
//   Type: "anchoredVolumeWeightedAveragePrice"
//   Name: "avwap"
//   Config:
//     candleInterval: "1 minutes"
//     multiplier: 1
//     price: "typical"
//     anchor:
//       - leftOperand: "timeOfDay"
//         operator: "equalTo"
//         rightOperand: "09:15:00"
//         conditionOperator: "and"
//
// Formula:
//
//   TypicalPrice = (High + Low + Close) / 3
//
//   AVWAP = Sum(Price * Volume) / Sum(Volume)
//
// Anchor:
//   Config-driven conditions.
//   Supports AND / OR combinations.
//   Accumulation resets on anchor trigger.
//
// Result:
//   ScalarResult:
//     Current
//     Previous
//     IsReady
//     Timestamp
//

type AVWAPAnchorCondition struct {
	LeftOperand       string
	Operator          string
	RightOperand      any
	ConditionOperator string
}

type AnchoredVolumeWeightedAveragePriceConfig struct {
	CandleInterval string
	Multiplier     float64
	Price          string
	Anchor         []AVWAPAnchorCondition
}

type AnchoredVolumeWeightedAveragePrice struct {
	id  string
	cfg AnchoredVolumeWeightedAveragePriceConfig

	priceVolumeSum float64
	volumeSum      float64

	lastTime time.Time

	anchorActive   bool
	previousAnchor bool

	hasResult bool
	lastValue float64

	result ScalarResult
}

func NewAnchoredVolumeWeightedAveragePrice(id string, cfg map[string]any) (*AnchoredVolumeWeightedAveragePrice, error) {
	if id == "" {
		return nil, fmt.Errorf("avwap: indicator id is required")
	}

	config := AnchoredVolumeWeightedAveragePriceConfig{
		CandleInterval: "1 minutes",
		Multiplier:     1,
		Price:          "typical",
	}

	if value, exists := cfg["candleInterval"]; exists {
		interval, ok := value.(string)
		if !ok || interval == "" {
			return nil, fmt.Errorf("avwap: invalid candleInterval")
		}
		config.CandleInterval = interval
	}

	if value, exists := cfg["multiplier"]; exists {
		switch v := value.(type) {
		case int:
			config.Multiplier = float64(v)
		case float64:
			config.Multiplier = v
		default:
			return nil, fmt.Errorf("avwap: multiplier must be numeric")
		}
	}

	if math.IsNaN(config.Multiplier) ||
		math.IsInf(config.Multiplier, 0) ||
		config.Multiplier <= 0 {
		return nil, fmt.Errorf("avwap: invalid multiplier")
	}

	if value, exists := cfg["price"]; exists {
		price, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("avwap: price must be a string")
		}
		config.Price = price
	}

	switch config.Price {
	case "typical", "close", "open", "high", "low", "median", "weighted":
	default:
		return nil, fmt.Errorf("avwap: unsupported price type %q", config.Price)
	}

	rawAnchor, exists := cfg["anchor"]
	if !exists {
		return nil, fmt.Errorf("avwap: anchor conditions required")
	}

	conditions, err := parseAVWAPAnchorConditions(rawAnchor)
	if err != nil {
		return nil, err
	}

	config.Anchor = conditions

	return &AnchoredVolumeWeightedAveragePrice{
		id:  id,
		cfg: config,
	}, nil
}

func (a *AnchoredVolumeWeightedAveragePrice) Update(candle core.MarketCandle) any {
	a.ingest(candle)
	return a.result
}

func (a *AnchoredVolumeWeightedAveragePrice) Result() any {
	return a.result
}

func (a *AnchoredVolumeWeightedAveragePrice) ingest(candle core.MarketCandle) {
	if !validAVWAPCandle(candle, a.lastTime) {
		a.result.IsReady = false
		return
	}

	timestamp := candle.GetTime()
	a.lastTime = timestamp

	// Step 1: Evaluate configured anchor conditions.
	anchorMatched := evaluateAVWAPAnchor(a.cfg.Anchor, candle)

	// Step 2: Rising-edge anchor trigger.
	anchorTriggered := anchorMatched && !a.previousAnchor
	a.previousAnchor = anchorMatched

	if anchorTriggered {
		a.priceVolumeSum = 0
		a.volumeSum = 0
		a.anchorActive = true
		a.hasResult = false
	}

	// Step 3: Wait for first anchor.
	if !a.anchorActive {
		a.result = ScalarResult{
			IsReady:   false,
			Timestamp: timestamp,
		}
		return
	}

	// Step 4: Select configured candle price.
	price := avwapCandlePrice(candle, a.cfg.Price)
	volume := candle.GetVolume()

	weightedPrice := price * volume

	if math.IsNaN(weightedPrice) ||
		math.IsInf(weightedPrice, 0) {
		a.result = ScalarResult{
			IsReady:   false,
			Timestamp: timestamp,
		}
		return
	}

	// Step 5: Accumulate price and volume.
	nextPV := a.priceVolumeSum + weightedPrice
	nextVolume := a.volumeSum + volume

	if math.IsNaN(nextPV) ||
		math.IsInf(nextPV, 0) ||
		math.IsNaN(nextVolume) ||
		math.IsInf(nextVolume, 0) {
		a.result = ScalarResult{
			IsReady:   false,
			Timestamp: timestamp,
		}
		return
	}

	a.priceVolumeSum = nextPV
	a.volumeSum = nextVolume

	// Step 6: Wait for positive accumulated volume.
	if a.volumeSum <= 0 {
		a.result = ScalarResult{
			IsReady:   false,
			Timestamp: timestamp,
		}
		return
	}

	// Step 7: Calculate AVWAP.
	current := a.priceVolumeSum / a.volumeSum

	// Multiplier is retained as configuration metadata.
	// Do not apply without confirmed QuantMan semantics.

	if math.IsNaN(current) || math.IsInf(current, 0) {
		a.result = ScalarResult{
			IsReady:   false,
			Timestamp: timestamp,
		}
		return
	}

	// Step 8: Previous ready value.
	previous := current

	if a.hasResult {
		previous = a.lastValue
	}

	// Step 9: Publish result.
	a.result = ScalarResult{
		IsReady:   true,
		Current:   current,
		Previous:  previous,
		Timestamp: timestamp,
	}

	a.lastValue = current
	a.hasResult = true
}

func parseAVWAPAnchorConditions(raw any) ([]AVWAPAnchorCondition, error) {
	var entries []map[string]any

	switch v := raw.(type) {
	case []map[string]any:
		entries = v
	case []any:
		for _, item := range v {
			entry, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("avwap: invalid anchor condition")
			}
			entries = append(entries, entry)
		}
	default:
		return nil, fmt.Errorf("avwap: anchor must be an array")
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("avwap: anchor cannot be empty")
	}

	conditions := make([]AVWAPAnchorCondition, 0, len(entries))

	for _, entry := range entries {
		left, ok := entry["leftOperand"].(string)
		if !ok || left == "" {
			return nil, fmt.Errorf("avwap: invalid leftOperand")
		}

		operator, ok := entry["operator"].(string)
		if !ok || operator == "" {
			return nil, fmt.Errorf("avwap: invalid operator")
		}

		right, exists := entry["rightOperand"]
		if !exists {
			return nil, fmt.Errorf("avwap: rightOperand required")
		}

		logical := "and"

		if value, exists := entry["conditionOperator"]; exists {
			v, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("avwap: invalid conditionOperator")
			}
			logical = strings.ToLower(v)
		}

		if logical != "and" && logical != "or" {
			return nil, fmt.Errorf("avwap: unsupported conditionOperator %q", logical)
		}

		if left != "timeOfDay" {
			return nil, fmt.Errorf("avwap: unsupported leftOperand %q; connect rule evaluator for other operands", left)
		}

		switch operator {
		case "equalTo", "isAbove", "isBelow",
			"equalOrAbove", "equalOrBelow":
		default:
			return nil, fmt.Errorf("avwap: unsupported operator %q", operator)
		}

		rightTime, ok := right.(string)
		if !ok {
			return nil, fmt.Errorf("avwap: timeOfDay rightOperand must be a string")
		}

		if _, err := parseAVWAPTime(rightTime); err != nil {
			return nil, err
		}

		conditions = append(conditions, AVWAPAnchorCondition{
			LeftOperand:       left,
			Operator:          operator,
			RightOperand:      right,
			ConditionOperator: logical,
		})
	}

	return conditions, nil
}

func evaluateAVWAPAnchor(conditions []AVWAPAnchorCondition, candle core.MarketCandle) bool {
	result := false

	for i, condition := range conditions {
		matched := evaluateAVWAPCondition(condition, candle)

		if i == 0 {
			result = matched
			continue
		}

		if conditions[i-1].ConditionOperator == "or" {
			result = result || matched
		} else {
			result = result && matched
		}
	}

	return result
}

func evaluateAVWAPCondition(condition AVWAPAnchorCondition, candle core.MarketCandle) bool {
	if condition.LeftOperand != "timeOfDay" {
		return false
	}

	currentTime := candle.GetTime()
	target, ok := condition.RightOperand.(string)

	if !ok {
		return false
	}

	targetSeconds, err := parseAVWAPTime(target)
	if err != nil {
		return false
	}

	currentSeconds := currentTime.Hour()*3600 +
		currentTime.Minute()*60 +
		currentTime.Second()

	switch condition.Operator {
	case "equalTo":
		return currentSeconds == targetSeconds
	case "isAbove":
		return currentSeconds > targetSeconds
	case "isBelow":
		return currentSeconds < targetSeconds
	case "equalOrAbove":
		return currentSeconds >= targetSeconds
	case "equalOrBelow":
		return currentSeconds <= targetSeconds
	default:
		return false
	}
}

func parseAVWAPTime(value string) (int, error) {
	parts := strings.Split(value, ":")

	if len(parts) != 2 && len(parts) != 3 {
		return 0, fmt.Errorf("avwap: invalid time %q", value)
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("avwap: invalid hour")
	}

	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("avwap: invalid minute")
	}

	second := 0

	if len(parts) == 3 {
		second, err = strconv.Atoi(parts[2])
		if err != nil {
			return 0, fmt.Errorf("avwap: invalid second")
		}
	}

	if hour < 0 || hour > 23 ||
		minute < 0 || minute > 59 ||
		second < 0 || second > 59 {
		return 0, fmt.Errorf("avwap: invalid time range")
	}

	return hour*3600 + minute*60 + second, nil
}

func avwapCandlePrice(candle core.MarketCandle, priceType string) float64 {
	open := candle.GetOpen()
	high := candle.GetHigh()
	low := candle.GetLow()
	closePrice := candle.GetClose()

	switch priceType {
	case "open":
		return open
	case "high":
		return high
	case "low":
		return low
	case "close":
		return closePrice
	case "median":
		return (high + low) / 2
	case "weighted":
		return (high + low + 2*closePrice) / 4
	default:
		return (high + low + closePrice) / 3
	}
}

func validAVWAPCandle(candle core.MarketCandle, lastTimestamp time.Time) bool {
	if candle == nil {
		return false
	}

	timestamp := candle.GetTime()

	if timestamp.IsZero() {
		return false
	}

	if !lastTimestamp.IsZero() && !timestamp.After(lastTimestamp) {
		return false
	}

	open := candle.GetOpen()
	high := candle.GetHigh()
	low := candle.GetLow()
	closePrice := candle.GetClose()
	volume := candle.GetVolume()

	for _, value := range []float64{
		open,
		high,
		low,
		closePrice,
	} {
		if math.IsNaN(value) ||
			math.IsInf(value, 0) ||
			value <= 0 {
			return false
		}
	}

	if math.IsNaN(volume) ||
		math.IsInf(volume, 0) ||
		volume < 0 {
		return false
	}

	if high < low {
		return false
	}

	if high < math.Max(open, closePrice) {
		return false
	}

	if low > math.Min(open, closePrice) {
		return false
	}

	return true
}
