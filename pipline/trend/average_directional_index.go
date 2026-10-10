package trend

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: Average Directional Index
//
// pipeConfigs: [{type: 'aDX', name: 'aDX', config: {period: <user supplied>, candleInterval: <user supplied>}}]
//
// Available result: Current, Previous, IsReady, Timestamp and indicator-specific fields in types.go.
// Entry Condition: Example - Previous <= 0 && Current > 0 (where meaningful); strategy decides entry.
// Exit Condition: Example - Previous >= 0 && Current < 0 (where meaningful); strategy decides exit.
// Original Formula: Average Directional Index; see independent calculation in ingest.
// Current is the latest value; Previous is the preceding ready value.
// First ready value requires configured warm-up candles; see ingest.
type ADXConfig struct {
	Period         int
	CandleInterval string
}

type ADX struct {
	id            string
	cfg           ADXConfig
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
	result        ADXResult
}

func NewADX(id string, cfg ADXConfig) (*ADX, error) {
	if id == "" {
		return nil, fmt.Errorf("indicator id is required")
	}
	if cfg.Period <= 0 {
		return nil, fmt.Errorf("ADX: period must be positive")
	}
	return &ADX{id: id, cfg: cfg}, nil
}
func (s *ADX) Update(candle core.MarketCandle) any { s.ingest(candle); return s.result }
func (s *ADX) Result() any                         { return s.result }
func (s *ADX) ingest(candle core.MarketCandle) {
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
	if len(closes) < period*2+1 {
		s.result.IsReady = false
		return
	}
	var dxValues []float64
	for end := period; end < len(closes); end++ {
		var trSum, plusSum, minusSum float64
		for i := end - period + 1; i <= end; i++ {
			up := highs[i] - highs[i-1]
			down := lows[i-1] - lows[i]
			trSum += math.Max(highs[i]-lows[i], math.Max(math.Abs(highs[i]-closes[i-1]), math.Abs(lows[i]-closes[i-1])))
			if up > down && up > 0 {
				plusSum += up
			}
			if down > up && down > 0 {
				minusSum += down
			}
		}
		plus, minus := 0.0, 0.0
		if trSum > 0 {
			plus = 100 * plusSum / trSum
			minus = 100 * minusSum / trSum
		}
		if plus+minus > 0 {
			dxValues = append(dxValues, 100*math.Abs(plus-minus)/(plus+minus))
		} else {
			dxValues = append(dxValues, 0)
		}
		if end == len(closes)-1 {
			s.result.PlusDI = plus
			s.result.MinusDI = minus
		}
	}
	if len(dxValues) < period {
		s.result.IsReady = false
		return
	}
	current = s.avg(dxValues[len(dxValues)-period:])
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
func (s *ADX) avg(values []float64) float64 {
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}
func (s *ADX) weighted(values []float64) float64 {
	var sum, den float64
	for i, v := range values {
		w := float64(i + 1)
		sum += v * w
		den += w
	}
	return sum / den
}
