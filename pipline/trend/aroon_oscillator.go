package trend

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: Aroon Oscillator
//
// pipeConfigs: [
//   {
//     type: 'aroonOscillator',
//     name: 'aroonOscillator',
//     config: {
//       period: 25,
//       candleInterval: '1 minutes',
//     },
//   },
// ],
//
// Available result:
//
//   - Aroon Oscillator
//   - Current
//   - Previous
//   - IsReady
//   - Timestamp
//
// Entry Condition:
//
//   Aroon Oscillator Is Above 0
//   Current > 0
//
//   Aroon Oscillator Crosses Above 0
//   Previous <= 0 && Current > 0
//
// Exit Condition:
//
//   Aroon Oscillator Is Below 0
//   Current < 0
//
//   Aroon Oscillator Crosses Below 0
//   Previous >= 0 && Current < 0
//
// Original Formula:
//
//   Aroon Up = 100 * (Period - PeriodsSinceHighestHigh) / Period
//
//   Aroon Down = 100 * (Period - PeriodsSinceLowestLow) / Period
//
//   Aroon Oscillator = Aroon Up - Aroon Down
//
// Explanation:
//
//   Current represents the latest calculated Aroon Oscillator.
//
//   Previous represents the preceding ready Aroon Oscillator.
//
//   Current > 0 indicates stronger upward trend positioning.
//
//   Current < 0 indicates stronger downward trend positioning.
//
//   Current = 0 indicates equal Aroon Up and Aroon Down.
//
//   IsReady becomes true after the configured warm-up period.
//
//   Period and CandleInterval are provided through configuration.
//
//   Entry and Exit conditions are examples.
//   The strategy determines which comparison to apply.


type AroonOscillatorConfig struct {
	Period         int
	CandleInterval string
}

type AroonOscillator struct {
	id            string
	cfg           AroonOscillatorConfig
	closes        []float64
	highs         []float64
	lows          []float64
	lastTimestamp time.Time
	seeded        bool
	hasResult     bool
	value         float64
	upper         float64
	lower         float64
	direction     int
	sar           float64
	extreme       float64
	accel         float64
	result        AroonOscillatorResult
}

func NewAroonOscillator(id string, cfg AroonOscillatorConfig) (*AroonOscillator, error) {
	if id == "" {
		return nil, fmt.Errorf("indicator id is required")
	}

	return &AroonOscillator{id: id, cfg: cfg}, nil
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
	timestamp, high, low, close := candle.GetTime(), candle.GetHigh(), candle.GetLow(), candle.GetClose()
	if timestamp.IsZero() || (!s.lastTimestamp.IsZero() && !timestamp.After(s.lastTimestamp)) || math.IsNaN(high) ||
		math.IsInf(high, 0) || math.IsNaN(low) || math.IsInf(low, 0) || math.IsNaN(close) || math.IsInf(close, 0) ||
		low <= 0 || high < low || close <= 0 {
		s.result.IsReady = false
		return
	}
	s.lastTimestamp = timestamp
	s.closes = append(s.closes, close)
	s.highs = append(s.highs, high)
	s.lows = append(s.lows, low)
	// Preserve a bounded window for memory efficiency while retaining sufficient warmup history.
	cfg := s.cfg
	maxHistory := max(500, 4*cfg.Period+10)
	if len(s.closes) > maxHistory {
		s.closes = s.closes[1:]
		s.highs = s.highs[1:]
		s.lows = s.lows[1:]
	}
	period := cfg.Period
	closes, highs, lows := s.closes, s.highs, s.lows
	if len(closes) < period {
		s.result.IsReady = false
		s.result.Timestamp = timestamp
		return
	}
	_ = cfg
	_ = high
	_ = low
	_ = highs
	_ = lows
	var current float64
	start := len(closes) - period
	maxIdx, minIdx := start, start
	for i := start + 1; i < len(closes); i++ {
		if highs[i] >= highs[maxIdx] {
			maxIdx = i
		}
		if lows[i] <= lows[minIdx] {
			minIdx = i
		}
	}

	up := 100 * float64(period-1-(len(closes)-1-maxIdx)) / float64(period-1)

	down := 100 * float64(period-1-(len(closes)-1-minIdx)) / float64(period-1)

	current = up - down

	if math.IsNaN(current) || math.IsInf(current, 0) {
		s.result.IsReady = false
		return
	}
	previous := current
	if s.hasResult {
		previous = s.value
	}
	s.result.IsReady = true
	s.result.Current = current
	s.result.Previous = previous
	s.result.Timestamp = timestamp
	s.value = current
	s.hasResult = true
}
func (s *AroonOscillator) avg(values []float64) float64 {
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}
func (s *AroonOscillator) weighted(values []float64) float64 {
	var sum, den float64
	for i, v := range values {
		w := float64(i + 1)
		sum += v * w
		den += w
	}
	return sum / den
}
