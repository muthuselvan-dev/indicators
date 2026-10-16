package trend

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// Simple Moving Average (SMA)
//
// Formula:
// SMA = Sum of last N closing prices / N
//
// Configuration:
//   period
//   candleInterval
//
// Result:
//   Current
//   Previous
//   IsReady
//   Timestamp

type SMAConfig struct {
	Period         int
	CandleInterval string
}

type SMA struct {
	id  string
	cfg SMAConfig

	closes []float64
	sum    float64

	lastTimestamp time.Time

	hasResult bool
	value     float64

	result SMAResult
}

func NewSMA(id string, cfg map[string]any) (*SMA, error) {

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
		return nil, fmt.Errorf("SMA: period must be positive")
	}

	return &SMA{
		id: id,
		cfg: SMAConfig{
			Period:         period,
			CandleInterval: candleInterval,
		},
		closes: make([]float64, 0, period),
	}, nil
}

func (s *SMA) Update(candle core.MarketCandle) any {

	s.ingest(candle)
	return s.result
}

func (s *SMA) Result() any {
	return s.result
}

func (s *SMA) ingest(candle core.MarketCandle) {

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

	period := s.cfg.Period

	// Add newest closing price.
	s.closes = append(s.closes, closePrice)
	s.sum += closePrice

	// Remove oldest closing price.
	if len(s.closes) > period {
		s.sum -= s.closes[0]
		s.closes = s.closes[1:]
	}

	// Warm-up period.
	if len(s.closes) < period {
		return
	}

	// Original SMA formula.
	current := s.sum / float64(period)

	if math.IsNaN(current) || math.IsInf(current, 0) {
		return
	}

	previous := current

	if s.hasResult {
		previous = s.value
	}

	s.result.Current = current
	s.result.Previous = previous
	s.result.Timestamp = timestamp
	s.result.IsReady = true

	s.value = current
	s.hasResult = true
}
