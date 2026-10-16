package trend

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// Aroon Oscillator = Aroon Up - Aroon Down

// Example usage:
// 	pipeConfigs := []pipline.Rule{
// 		{
// 			Type: "aroonOscillator",
// 			Name: "aroonOscillator",
// 			Config: map[string]any{
// 				"period":          14,
// 				"candleInterval": "1 minutes",
// 			},
// 		},
// 	}

// Entry (BUY): Aroon Oscillator is above 0
// Exit (SELL): Aroon Oscillator is below 0
//

// Default period: 14
// Default candle interval: 1 minutes
//
// Result:
// Current, Previous, IsReady, Timestamp

type AroonOscillator struct {
	id             string
	period         int
	candleInterval string

	highs []float64
	lows  []float64

	lastTimestamp time.Time

	hasResult bool
	value     float64

	result AroonOscillatorResult
}

func NewAroonOscillator(id string, cfg map[string]any) (*AroonOscillator, error) {

	if id == "" {
		return nil, fmt.Errorf("indicator id is required")
	}

	period := 14
	candleInterval := "1 minutes"

	if value, ok := cfg["period"].(int); ok {
		period = value
	}

	if value, ok := cfg["candleInterval"].(string); ok {
		candleInterval = value
	}

	return &AroonOscillator{
		id:             id,
		period:         period,
		candleInterval: candleInterval,
	}, nil
}

func (s *AroonOscillator) Update(candle core.MarketCandle) any {
	s.ingest(candle)
	return s.result
}

func (s *AroonOscillator) Result() any {
	return s.result
}

func (s *AroonOscillator) ingest(candle core.MarketCandle) {
	if candle == nil {
		return
	}

	timestamp := candle.GetTime()
	high := candle.GetHigh()
	low := candle.GetLow()
	closePrice := candle.GetClose()

	// Validate candle data.
	if timestamp.IsZero() ||
		(!s.lastTimestamp.IsZero() &&
			!timestamp.After(s.lastTimestamp)) ||
		math.IsNaN(high) ||
		math.IsInf(high, 0) ||
		math.IsNaN(low) ||
		math.IsInf(low, 0) ||
		math.IsNaN(closePrice) ||
		math.IsInf(closePrice, 0) ||
		low <= 0 ||
		high < low ||
		closePrice <= 0 {

		s.result.IsReady = false
		return
	}

	s.lastTimestamp = timestamp

	// Store candle highs and lows.
	s.highs = append(s.highs, high)
	s.lows = append(s.lows, low)

	// Maintain rolling window.
	if len(s.highs) > s.period {
		s.highs = s.highs[1:]
		s.lows = s.lows[1:]
	}

	// Warm-up.
	if len(s.highs) < s.period {
		s.result.IsReady = false
		s.result.Timestamp = timestamp
		return
	}

	// Find highest high and lowest low indices.
	maxIdx := 0
	minIdx := 0

	for i := 1; i < len(s.highs); i++ {
		if s.highs[i] >= s.highs[maxIdx] {
			maxIdx = i
		}

		if s.lows[i] <= s.lows[minIdx] {
			minIdx = i
		}
	}

	// Same formula as your existing implementation.
	periodsSinceHigh := len(s.highs) - 1 - maxIdx
	periodsSinceLow := len(s.lows) - 1 - minIdx

	up := 100 *
		float64(s.period-1-periodsSinceHigh) /
		float64(s.period-1)

	down := 100 *
		float64(s.period-1-periodsSinceLow) /
		float64(s.period-1)

	current := up - down

	if math.IsNaN(current) || math.IsInf(current, 0) {
		s.result.IsReady = false
		s.result.Timestamp = timestamp
		return
	}

	previous := current
	if s.hasResult {
		previous = s.value
	}

	s.result = AroonOscillatorResult{
		IsReady:   true,
		Current:   current,
		Previous:  previous,
		Timestamp: timestamp,
	}

	s.value = current
	s.hasResult = true
}
