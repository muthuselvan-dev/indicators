package trend

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: Parabolic SAR
//
// pipeConfigs: [{type: 'parabolicSAR', name: 'parabolicSAR', config: {period: <user supplied>, candleInterval: <user supplied>}}]
//
// Available result: Current, Previous, IsReady, Timestamp and indicator-specific fields in types.go.
// Entry Condition: Example - Previous <= 0 && Current > 0 (where meaningful); strategy decides entry.
// Exit Condition: Example - Previous >= 0 && Current < 0 (where meaningful); strategy decides exit.
// Original Formula: Parabolic SAR; see independent calculation in ingest.
// Current is the latest value; Previous is the preceding ready value.
// First ready value requires configured warm-up candles; see ingest.
type ParabolicSARConfig struct {
	CandleInterval string
	Step           float64
	MaxStep        float64
}

type ParabolicSAR struct {
	id            string
	cfg           ParabolicSARConfig
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
	result        ParabolicSARResult
}

func NewParabolicSAR(id string, cfg ParabolicSARConfig) (*ParabolicSAR, error) {
	if id == "" {
		return nil, fmt.Errorf("indicator id is required")
	}
	if cfg.Step <= 0 || cfg.MaxStep < cfg.Step {
		return nil, fmt.Errorf("invalid step or maxStep")
	}
	return &ParabolicSAR{id: id, cfg: cfg}, nil
}
func (s *ParabolicSAR) Update(candle core.MarketCandle) any { s.ingest(candle); return s.result }
func (s *ParabolicSAR) Result() any                         { return s.result }
func (s *ParabolicSAR) ingest(candle core.MarketCandle) {
	if candle == nil {
		return
	}
	timestamp, high, low, close := candle.GetTime(), candle.GetHigh(), candle.GetLow(), candle.GetClose()
	if timestamp.IsZero() || (!s.lastTimestamp.IsZero() && !timestamp.After(s.lastTimestamp)) || math.IsNaN(high) || math.IsInf(high, 0) || math.IsNaN(low) || math.IsInf(low, 0) || math.IsNaN(close) || math.IsInf(close, 0) || low <= 0 || high < low || close <= 0 {
		s.result.IsReady = false
		return
	}
	s.lastTimestamp = timestamp
	s.closes = append(s.closes, close)
	s.highs = append(s.highs, high)
	s.lows = append(s.lows, low)
	// Preserve a bounded window for memory efficiency while retaining sufficient warmup history.
	cfg := s.cfg
	maxHistory := max(500, 4*2+10)
	if len(s.closes) > maxHistory {
		s.closes = s.closes[1:]
		s.highs = s.highs[1:]
		s.lows = s.lows[1:]
	}
	period := 2
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
	if !s.seeded {
		s.direction = 1
		s.sar = lows[len(lows)-2]
		s.extreme = highs[len(highs)-2]
		s.accel = cfg.Step
		s.seeded = true
	}
	current = s.sar + s.accel*(s.extreme-s.sar)
	if s.direction > 0 {
		current = math.Min(current, lows[len(lows)-2])
		if len(lows) > 2 {
			current = math.Min(current, lows[len(lows)-3])
		}
		if low < current {
			s.direction = -1
			current = s.extreme
			s.extreme = low
			s.accel = cfg.Step
		} else if high > s.extreme {
			s.extreme = high
			s.accel = math.Min(cfg.MaxStep, s.accel+cfg.Step)
		}
	} else {
		current = math.Max(current, highs[len(highs)-2])
		if len(highs) > 2 {
			current = math.Max(current, highs[len(highs)-3])
		}
		if high > current {
			s.direction = 1
			current = s.extreme
			s.extreme = high
			s.accel = cfg.Step
		} else if low < s.extreme {
			s.extreme = low
			s.accel = math.Min(cfg.MaxStep, s.accel+cfg.Step)
		}
	}
	s.sar = current
	s.result.Direction = s.direction
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
func (s *ParabolicSAR) avg(values []float64) float64 {
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}
func (s *ParabolicSAR) weighted(values []float64) float64 {
	var sum, den float64
	for i, v := range values {
		w := float64(i + 1)
		sum += v * w
		den += w
	}
	return sum / den
}
