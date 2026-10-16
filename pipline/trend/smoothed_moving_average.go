package trend

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// Indicator Name: Smoothed Moving Average (SMMA)
//
// Pipe Config:
// type: "smoothedMovingAverage"
// name: "smma"
// config:
//   period: 7
//   candleInterval: "1 minutes"
//
// Entry Condition:
//   BUY when Close crosses above SMMA
//   Previous Close <= Previous SMMA
//   Current Close > Current SMMA
//
// Exit Condition:
//   SELL when Close crosses below SMMA
//   Previous Close >= Previous SMMA
//   Current Close < Current SMMA
//
// Original Formula:
//
// First SMMA:
//   SMMA = Sum(Close, Period) / Period
//
// Subsequent SMMA:
//   SMMA = (PreviousSMMA * (Period - 1) + Close) / Period
//
// Result:
//   Current
//   Previous
//   IsReady
//   Timestamp

type SMMAConfig struct {
	Period         int
	CandleInterval string
}

type SMMA struct {
	id  string
	cfg SMMAConfig

	lastTimestamp time.Time

	count int
	sum   float64

	hasResult bool
	value     float64

	result SMMAResult
}

func NewSMMA(id string, cfg map[string]any) (*SMMA, error) {
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
		return nil, fmt.Errorf("SMMA: period must be positive")
	}

	return &SMMA{
		id: id,
		cfg: SMMAConfig{
			Period:         period,
			CandleInterval: candleInterval,
		},
	}, nil
}

func (s *SMMA) Update(candle core.MarketCandle) any {
	s.ingest(candle)
	return s.result
}

func (s *SMMA) Result() any {
	return s.result
}

func (s *SMMA) ingest(candle core.MarketCandle) {
	s.result.IsReady = false

	if candle == nil {
		return
	}

	timestamp := candle.GetTime()
	high := candle.GetHigh()
	low := candle.GetLow()
	closePrice := candle.GetClose()

	if timestamp.IsZero() ||
		(!s.lastTimestamp.IsZero() && !timestamp.After(s.lastTimestamp)) ||
		math.IsNaN(high) || math.IsInf(high, 0) ||
		math.IsNaN(low) || math.IsInf(low, 0) ||
		math.IsNaN(closePrice) || math.IsInf(closePrice, 0) ||
		low <= 0 || high < low || closePrice <= 0 {
		return
	}

	s.lastTimestamp = timestamp
	s.result.Timestamp = timestamp

	period := s.cfg.Period

	// First period candles: calculate SMA seed.
	if !s.hasResult {
		s.sum += closePrice
		s.count++

		if s.count < period {
			return
		}

		s.value = s.sum / float64(period)
		s.hasResult = true

		s.result.Current = s.value
		s.result.Previous = s.value
		s.result.IsReady = true
		return
	}

	// Original SMMA / Wilder smoothing formula.
	previous := s.value

	current := (previous*float64(period-1) + closePrice) / float64(period)

	if math.IsNaN(current) || math.IsInf(current, 0) {
		return
	}

	s.value = current

	s.result.Current = current
	s.result.Previous = previous
	s.result.IsReady = true
	s.result.Timestamp = timestamp
}
