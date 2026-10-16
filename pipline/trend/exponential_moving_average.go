package trend

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: Exponential Moving Average (EMA)
//
// pipeConfigs:
// [
//   {
//     type: 'eMA',
//     name: 'eMA',
//     config: {
//       period: 9,
//       candleInterval: '1 minutes'
//     }
//   }
// ]
//
// Original Formula:
//
// Multiplier = 2 / (Period + 1)
//
// Initial EMA = SMA of first Period closing prices
//
// EMA = PreviousEMA +
//       Multiplier * (CurrentClose - PreviousEMA)
//
// Available Result:
//   - Current
//   - Previous
//   - IsReady
//   - Timestamp
//
// Entry/Exit conditions are handled by the strategy.

type EMAConfig struct {
	Period         int
	CandleInterval string
}

type EMA struct {
	id  string
	cfg EMAConfig

	lastTimestamp time.Time

	// Initial SMA warm-up
	count int
	sum   float64

	// EMA state
	hasResult bool
	value     float64

	result EMAResult
}

func NewEMA(id string, cfg map[string]any) (*EMA, error) {
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
		return nil, fmt.Errorf("EMA: period must be positive")
	}

	return &EMA{
		id: id,
		cfg: EMAConfig{
			Period:         period,
			CandleInterval: candleInterval,
		},
	}, nil
}

func (s *EMA) Update(candle core.MarketCandle) any {
	s.ingest(candle)
	return s.result
}

func (s *EMA) Result() any {
	return s.result
}

func (s *EMA) ingest(candle core.MarketCandle) {
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
	s.result.Timestamp = timestamp
	s.result.IsReady = false

	period := s.cfg.Period

	// Warm-up: calculate initial SMA.
	if !s.hasResult {
		s.sum += closePrice
		s.count++

		if s.count < period {
			return
		}

		current := s.sum / float64(period)

		s.result.Current = current
		s.result.Previous = current
		s.result.IsReady = true

		s.value = current
		s.hasResult = true

		return
	}

	// Standard EMA calculation.
	multiplier := 2.0 / float64(period+1)

	previous := s.value

	current := previous +
		multiplier*(closePrice-previous)

	if math.IsNaN(current) || math.IsInf(current, 0) {
		return
	}

	s.result.Current = current
	s.result.Previous = previous
	s.result.IsReady = true

	s.value = current
}
