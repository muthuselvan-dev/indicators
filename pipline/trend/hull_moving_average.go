package trend

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// Hull Moving Average (HMA)
//
// Formula:
//
// HMA(n) = WMA(
//     2 * WMA(Close, n/2) - WMA(Close, n),
//     sqrt(n),
// )
//
// WMA assigns larger weights to newer prices.
//
// Available Result:
//   Current
//   Previous
//   IsReady
//   Timestamp
//
// Entry and exit conditions are strategy-specific.

type HMA struct {
	id             string
	Period         int
	CandleInterval string

	closes []float64

	lastTimestamp time.Time

	hasResult bool
	value     float64

	result HMAResult
}

func NewHMA(id string, cfg map[string]any) (*HMA, error) {
	if id == "" {
		return nil, fmt.Errorf("indicator id is required")
	}

	period := 9
	candleInterval := "1 minutes"

	if value, ok := cfg["period"].(int); ok {
		period = value
	}

	if value, ok := cfg["candleInterval"].(string); ok {
		candleInterval = value
	}

	if period <= 0 {
		return nil, fmt.Errorf("HMA: period must be positive")
	}

	return &HMA{
		id:             id,
		Period:         period,
		CandleInterval: candleInterval,
	}, nil
}

func (s *HMA) Update(candle core.MarketCandle) any {
	s.ingest(candle)
	return s.result
}

func (s *HMA) Result() any {
	return s.result
}

func (s *HMA) ingest(candle core.MarketCandle) {
	if candle == nil {
		return
	}

	timestamp := candle.GetTime()
	high := candle.GetHigh()
	low := candle.GetLow()
	closePrice := candle.GetClose()

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
	s.result.Timestamp = timestamp
	s.result.IsReady = false

	period := s.Period

	half := period / 2
	if half < 1 {
		half = 1
	}

	root := int(math.Sqrt(float64(period)))
	if root < 1 {
		root = 1
	}

	// Only retain candles needed for HMA.
	maxHistory := period + root - 1

	s.closes = append(s.closes, closePrice)

	if len(s.closes) > maxHistory {
		s.closes = s.closes[1:]
	}

	if len(s.closes) < maxHistory {
		return
	}

	// Calculate the transformed series:
	//
	// 2 * WMA(half period) - WMA(full period)

	transformed := make([]float64, 0, root)

	for i := len(s.closes) - root; i < len(s.closes); i++ {
		window := s.closes[i-period+1 : i+1]

		halfWMA := weightedAverage(
			window[period-half:],
		)

		fullWMA := weightedAverage(window)

		transformed = append(
			transformed,
			2*halfWMA-fullWMA,
		)
	}

	// Final Hull Moving Average.
	current := weightedAverage(transformed)

	if math.IsNaN(current) || math.IsInf(current, 0) {
		return
	}

	previous := current
	if s.hasResult {
		previous = s.value
	}

	s.result.Current = current
	s.result.Previous = previous
	s.result.IsReady = true
	s.result.Timestamp = timestamp

	s.value = current
	s.hasResult = true
}

// weightedAverage calculates a standard
// linearly weighted moving average.
//
// Oldest value receives weight 1.
// Newest value receives weight N.
func weightedAverage(values []float64) float64 {
	var numerator float64
	var denominator float64

	for i, value := range values {
		weight := float64(i + 1)

		numerator += value * weight
		denominator += weight
	}

	return numerator / denominator
}
