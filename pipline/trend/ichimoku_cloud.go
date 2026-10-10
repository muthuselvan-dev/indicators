package trend

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: Ichimoku Cloud
//
// pipeConfigs: [{type: 'ichimokuCloud', name: 'ichimokuCloud', config: {period: <user supplied>, candleInterval: <user supplied>}}]
//
// Available result: Current, Previous, IsReady, Timestamp and indicator-specific fields in types.go.
// Entry Condition: Example - Previous <= 0 && Current > 0 (where meaningful); strategy decides entry.
// Exit Condition: Example - Previous >= 0 && Current < 0 (where meaningful); strategy decides exit.
// Original Formula: Ichimoku Cloud; see independent calculation in ingest.
// Current is the latest value; Previous is the preceding ready value.
// First ready value requires configured warm-up candles; see ingest.
type IchimokuCloudConfig struct {
	Period           int
	CandleInterval   string
	ConversionPeriod int
	BasePeriod       int
	SpanBPeriod      int
	Displacement     int
}

type IchimokuCloud struct {
	id            string
	cfg           IchimokuCloudConfig
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
	result        IchimokuCloudResult
}

func NewIchimokuCloud(id string, cfg IchimokuCloudConfig) (*IchimokuCloud, error) {
	if id == "" {
		return nil, fmt.Errorf("indicator id is required")
	}
	if cfg.ConversionPeriod <= 0 || cfg.BasePeriod <= 0 || cfg.SpanBPeriod <= 0 || cfg.Displacement < 0 {
		return nil, fmt.Errorf("invalid ichimoku periods")
	}
	return &IchimokuCloud{id: id, cfg: cfg}, nil
}
func (s *IchimokuCloud) Update(candle core.MarketCandle) any { s.ingest(candle); return s.result }
func (s *IchimokuCloud) Result() any                         { return s.result }
func (s *IchimokuCloud) ingest(candle core.MarketCandle) {
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
	maxHistory := max(500, 4*max(cfg.ConversionPeriod, max(cfg.BasePeriod, cfg.SpanBPeriod))+10)
	if len(s.closes) > maxHistory {
		s.closes = s.closes[1:]
		s.highs = s.highs[1:]
		s.lows = s.lows[1:]
	}
	period := max(cfg.ConversionPeriod, max(cfg.BasePeriod, cfg.SpanBPeriod))
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
	midrange := func(count int) float64 {
		start := len(highs) - count
		hi, lo := highs[start], lows[start]
		for i := start + 1; i < len(highs); i++ {
			hi = math.Max(hi, highs[i])
			lo = math.Min(lo, lows[i])
		}
		return (hi + lo) / 2
	}
	conversion := midrange(cfg.ConversionPeriod)
	base := midrange(cfg.BasePeriod)
	spanB := midrange(cfg.SpanBPeriod)
	spanA := (conversion + base) / 2
	current = spanA
	s.result.Conversion = conversion
	s.result.Base = base
	s.result.SpanA = spanA
	s.result.SpanB = spanB
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
func (s *IchimokuCloud) avg(values []float64) float64 {
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}
func (s *IchimokuCloud) weighted(values []float64) float64 {
	var sum, den float64
	for i, v := range values {
		w := float64(i + 1)
		sum += v * w
		den += w
	}
	return sum / den
}
