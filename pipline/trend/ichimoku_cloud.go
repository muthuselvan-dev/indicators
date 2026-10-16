package trend

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// Ichimoku Cloud
//
// QuantMan configuration:
//   conversionPeriod: 9
//   basePeriod:       26
//   spanPeriod:       52
//   displacement:     26
//   candleInterval:   "1 minutes"
//
// Formulas:
//
// Conversion = (HighestHigh(9) + LowestLow(9)) / 2
// Base       = (HighestHigh(26) + LowestLow(26)) / 2
// SpanA      = (Conversion + Base) / 2
// SpanB      = (HighestHigh(52) + LowestLow(52)) / 2
//
// SpanA and SpanB are plotted forward by displacement.
// Chikou Span is Close plotted backward by displacement.
//
// This implementation returns the values calculated at
// the current candle. It does not shift timestamps.
//
// Current maps to SpanA for compatibility with the
// existing IchimokuCloudResult structure.
//
// QuantMan's exact Current mapping and displacement
// semantics still require parity verification.

type IchimokuCloudConfig struct {
	ConversionPeriod int
	BasePeriod       int
	SpanPeriod       int
	Displacement     int
	CandleInterval   string
}

type IchimokuCloud struct {
	id  string
	cfg IchimokuCloudConfig

	highs []float64
	lows  []float64

	lastTimestamp time.Time

	hasResult bool
	value     float64

	result IchimokuCloudResult
}

func NewIchimokuCloud(id string, cfg map[string]any) (*IchimokuCloud, error) {
	if id == "" {
		return nil, fmt.Errorf("indicator id is required")
	}

	config := IchimokuCloudConfig{
		ConversionPeriod: 9,
		BasePeriod:       26,
		SpanPeriod:       52,
		Displacement:     26,
		CandleInterval:   "1 minutes",
	}

	if value, ok := cfg["conversionPeriod"].(int); ok {
		config.ConversionPeriod = value
	}

	if value, ok := cfg["basePeriod"].(int); ok {
		config.BasePeriod = value
	}

	if value, ok := cfg["spanPeriod"].(int); ok {
		config.SpanPeriod = value
	}

	if value, ok := cfg["displacement"].(int); ok {
		config.Displacement = value
	}

	if value, ok := cfg["candleInterval"].(string); ok {
		config.CandleInterval = value
	}

	if config.ConversionPeriod <= 0 ||
		config.BasePeriod <= 0 ||
		config.SpanPeriod <= 0 ||
		config.Displacement < 0 {
		return nil, fmt.Errorf("invalid Ichimoku configuration")
	}

	return &IchimokuCloud{
		id:  id,
		cfg: config,
	}, nil
}

func (s *IchimokuCloud) Update(candle core.MarketCandle) any {
	s.ingest(candle)
	return s.result
}

func (s *IchimokuCloud) Result() any {
	return s.result
}

func (s *IchimokuCloud) ingest(
	candle core.MarketCandle,
) {
	if candle == nil {
		s.result.IsReady = false
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

		s.result.IsReady = false
		return
	}

	s.lastTimestamp = timestamp
	s.result.Timestamp = timestamp
	s.result.IsReady = false

	s.highs = append(s.highs, high)
	s.lows = append(s.lows, low)

	// Determine required history.
	maxPeriod := s.cfg.ConversionPeriod

	if s.cfg.BasePeriod > maxPeriod {
		maxPeriod = s.cfg.BasePeriod
	}

	if s.cfg.SpanPeriod > maxPeriod {
		maxPeriod = s.cfg.SpanPeriod
	}

	// Keep only the history required for calculations.
	if len(s.highs) > maxPeriod {
		s.highs = s.highs[1:]
		s.lows = s.lows[1:]
	}

	if len(s.highs) < maxPeriod {
		return
	}

	conversion := s.midrange(s.cfg.ConversionPeriod)
	base := s.midrange(s.cfg.BasePeriod)
	spanB := s.midrange(s.cfg.SpanPeriod)

	spanA := (conversion + base) / 2.0

	current := spanA

	if math.IsNaN(current) || math.IsInf(current, 0) {
		return
	}

	previous := current

	if s.hasResult {
		previous = s.value
	}

	s.result.Conversion = conversion
	s.result.Base = base
	s.result.SpanA = spanA
	s.result.SpanB = spanB

	s.result.Current = current
	s.result.Previous = previous
	s.result.IsReady = true
	s.result.Timestamp = timestamp

	s.value = current
	s.hasResult = true
}

// midrange returns:
//
// (Highest High + Lowest Low) / 2
//
// over the most recent period candles.
func (s *IchimokuCloud) midrange(period int) float64 {
	start := len(s.highs) - period

	highest := s.highs[start]
	lowest := s.lows[start]

	for i := start + 1; i < len(s.highs); i++ {
		if s.highs[i] > highest {
			highest = s.highs[i]
		}

		if s.lows[i] < lowest {
			lowest = s.lows[i]
		}
	}

	return (highest + lowest) / 2.0
}
