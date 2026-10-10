package trend

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: Super Trend
//
// pipeConfigs: [{type: 'supertrend', name: 'supertrend', config: {noOfCandles: <user supplied>, candleInterval: <user supplied>}}]
//
// Available result: Current, Previous, IsReady, Timestamp and indicator-specific fields in types.go.
// Entry Condition: Example - Previous <= 0 && Current > 0 (where meaningful); strategy decides entry.
// Exit Condition: Example - Previous >= 0 && Current < 0 (where meaningful); strategy decides exit.
// Original Formula: Super Trend; see independent calculation in ingest.
// Current is the latest value; Previous is the preceding ready value.
// First ready value requires configured warm-up candles; see ingest.
type SupertrendConfig struct {
	Period         int
	CandleInterval string
	Multiplier     float64
}

type Supertrend struct {
	id            string
	cfg           SupertrendConfig
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
	result        SupertrendResult
}

func NewSupertrend(id string, cfg SupertrendConfig) (*Supertrend, error) {
	if id == "" {
		return nil, fmt.Errorf("indicator id is required")
	}
	if cfg.Period <= 0 {
		return nil, fmt.Errorf("Supertrend: period must be positive")
	}
	if cfg.Multiplier <= 0 {
		return nil, fmt.Errorf("multiplier must be positive")
	}
	return &Supertrend{id: id, cfg: cfg}, nil
}
func (s *Supertrend) Update(candle core.MarketCandle) any { s.ingest(candle); return s.result }
func (s *Supertrend) Result() any                         { return s.result }
func (s *Supertrend) ingest(candle core.MarketCandle) {
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
	atrPeriod := period
	if len(closes) < atrPeriod+1 {
		s.result.IsReady = false
		return
	}
	var atr float64
	for i := len(closes) - atrPeriod; i < len(closes); i++ {
		tr := math.Max(highs[i]-lows[i], math.Max(math.Abs(highs[i]-closes[i-1]), math.Abs(lows[i]-closes[i-1])))
		atr += tr
	}
	atr /= float64(atrPeriod)
	mid := (high + low) / 2
	upper := mid + cfg.Multiplier*atr
	lower := mid - cfg.Multiplier*atr
	if s.seeded {
		if upper > s.upper && s.direction < 0 {
			upper = s.upper
		}
		if lower < s.lower && s.direction > 0 {
			lower = s.lower
		}
	}
	direction := s.direction
	if !s.seeded {
		direction = 1
	} else if close > s.upper {
		direction = 1
	} else if close < s.lower {
		direction = -1
	}
	current = lower
	if direction < 0 {
		current = upper
	}
	s.upper = upper
	s.lower = lower
	s.direction = direction
	s.seeded = true
	s.result.UpperBand = upper
	s.result.LowerBand = lower
	s.result.Direction = direction
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
func (s *Supertrend) avg(values []float64) float64 {
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}
func (s *Supertrend) weighted(values []float64) float64 {
	var sum, den float64
	for i, v := range values {
		w := float64(i + 1)
		sum += v * w
		den += w
	}
	return sum / den
}
