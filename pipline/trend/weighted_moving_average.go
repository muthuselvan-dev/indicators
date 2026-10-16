package trend

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// Indicator Name: Weighted Moving Average (WMA)
//
// Pipe Config:
//   type: "weightedMovingAverage"
//   name: "weightedMovingAverage"
//   config:
//     period: 9
//     candleInterval: "1 minutes"
//
// Entry Condition (example):
//   BUY when price crosses above WMA.
//   PreviousClose <= PreviousWMA && CurrentClose > CurrentWMA
//
// Exit Condition (example):
//   SELL when price crosses below WMA.
//   PreviousClose >= PreviousWMA && CurrentClose < CurrentWMA
//
// Original Formula:
//   WMA = Sum(Close[i] * Weight[i]) / Sum(Weight[i])
//
//   Weights = 1, 2, 3, ..., Period
//   Oldest candle gets weight 1.
//   Latest candle gets weight Period.
//
//   Denominator = Period * (Period + 1) / 2
//
// Result:
//   Current, Previous, IsReady, Timestamp
//
// First ready value requires Period valid candles.

type WMAConfig struct {
	Period         int
	CandleInterval string
}

type WMA struct {
	id  string
	cfg WMAConfig

	closes        []float64
	lastTimestamp time.Time

	hasResult bool
	value     float64

	result WMAResult
}

func NewWMA(
	id string,
	cfg map[string]any,
) (*WMA, error) {
	if id == "" {
		return nil, fmt.Errorf("indicator id is required")
	}

	config := WMAConfig{
		Period:         9,
		CandleInterval: "1 minutes",
	}

	if v, ok := cfg["period"]; ok {
		period, valid := v.(int)
		if !valid {
			return nil, fmt.Errorf(
				"WMA: period must be an integer",
			)
		}
		config.Period = period
	}

	if v, ok := cfg["candleInterval"]; ok {
		interval, valid := v.(string)
		if !valid || interval == "" {
			return nil, fmt.Errorf(
				"WMA: candleInterval must be a non-empty string",
			)
		}
		config.CandleInterval = interval
	}

	if config.Period <= 0 {
		return nil, fmt.Errorf(
			"WMA: period must be positive",
		)
	}

	return &WMA{
		id:     id,
		cfg:    config,
		closes: make([]float64, 0, config.Period),
	}, nil
}

func (s *WMA) Update(candle core.MarketCandle) any {
	s.ingest(candle)
	return s.result
}

func (s *WMA) Result() any {
	return s.result
}

func (s *WMA) ingest(candle core.MarketCandle) {
	s.result.IsReady = false

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
		return
	}

	s.lastTimestamp = timestamp
	s.result.Timestamp = timestamp

	s.closes = append(s.closes, closePrice)

	period := s.cfg.Period

	// Keep only the required period candles.
	if len(s.closes) > period {
		s.closes = s.closes[1:]
	}

	// Warm-up.
	if len(s.closes) < period {
		return
	}

	// Original Weighted Moving Average.
	var weightedSum float64

	for i, price := range s.closes {
		weight := float64(i + 1)
		weightedSum += price * weight
	}

	denominator := float64(period) *
		float64(period+1) / 2.0

	current := weightedSum / denominator

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
