package trend

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// Indicator Name: Wilder Smoothing Average (WSA / RMA)
//
// Pipe Config:
//   type: "wilderSmoothingAverage"
//   name: "wilderSmoothingAverage"
//   config:
//     period: 14
//     candleInterval: "1 minutes"
//
// Entry Condition (example):
//   BUY when price crosses above Wilder Average.
//   PreviousClose <= PreviousWSA && CurrentClose > CurrentWSA
//
// Exit Condition (example):
//   SELL when price crosses below Wilder Average.
//   PreviousClose >= PreviousWSA && CurrentClose < CurrentWSA
//
// Original Formula:
//
// First ready value:
//   WSA = SMA(Close, Period)
//
// Subsequent values:
//   WSA = (PreviousWSA * (Period - 1) + Close) / Period
//
// Equivalent smoothing coefficient:
//   Alpha = 1 / Period
//
// Result:
//   Current, Previous, IsReady, Timestamp
//
// Warm-up:
//   First ready value after Period valid candles.
//
// Note:
// This implements standard Wilder smoothing.
// Exact QuantMan parity must be verified with backtest data.

type WilderSmoothingAverageConfig struct {
	Period         int
	CandleInterval string
}

type WilderSmoothingAverage struct {
	id  string
	cfg WilderSmoothingAverageConfig

	lastTimestamp time.Time

	count  int
	sum    float64
	value  float64
	seeded bool

	result WilderSmoothingAverageResult
}

func NewWilderSmoothingAverage(
	id string,
	cfg map[string]any,
) (*WilderSmoothingAverage, error) {
	if id == "" {
		return nil, fmt.Errorf("indicator id is required")
	}

	config := WilderSmoothingAverageConfig{
		Period:         14,
		CandleInterval: "1 minutes",
	}

	if v, exists := cfg["period"]; exists {
		period, ok := v.(int)
		if !ok {
			return nil, fmt.Errorf(
				"WilderSmoothingAverage: period must be an integer",
			)
		}
		config.Period = period
	}

	if v, exists := cfg["candleInterval"]; exists {
		interval, ok := v.(string)
		if !ok || interval == "" {
			return nil, fmt.Errorf(
				"WilderSmoothingAverage: candleInterval must be a non-empty string",
			)
		}
		config.CandleInterval = interval
	}

	if config.Period <= 0 {
		return nil, fmt.Errorf(
			"WilderSmoothingAverage: period must be positive",
		)
	}

	return &WilderSmoothingAverage{
		id:  id,
		cfg: config,
	}, nil
}

func (s *WilderSmoothingAverage) Update(
	candle core.MarketCandle,
) any {
	s.ingest(candle)
	return s.result
}

func (s *WilderSmoothingAverage) Result() any {
	return s.result
}

func (s *WilderSmoothingAverage) ingest(
	candle core.MarketCandle,
) {
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

	s.count++

	period := s.cfg.Period

	var current float64

	if !s.seeded {
		// Initial SMA seed.
		s.sum += closePrice

		if s.count < period {
			return
		}

		current = s.sum / float64(period)
		s.seeded = true
	} else {
		// Wilder recursive smoothing.
		current = (s.value*float64(period-1) + closePrice) / float64(period)
	}

	if math.IsNaN(current) ||
		math.IsInf(current, 0) {
		return
	}

	previous := current

	if s.count > period {
		previous = s.value
	}

	s.result.Current = current
	s.result.Previous = previous
	s.result.IsReady = true
	s.result.Timestamp = timestamp

	s.value = current
}
