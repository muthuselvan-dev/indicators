package trend

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// Indicator Name: Supertrend
//
// Pipe Config:
//   type: "superTrend"
//   name: "superTrend"
//   config:
//     period: 10
//     multiplier: 2
//     candleInterval: "1 minutes"
//
// Entry Condition (example):
//   BUY when trend changes from downtrend to uptrend.
//   Previous Direction == -1 && Current Direction == 1
//
// Exit Condition (example):
//   SELL when trend changes from uptrend to downtrend.
//   Previous Direction == 1 && Current Direction == -1
//
// Original Formula:
//
// True Range:
//   TR = max(High-Low, abs(High-PreviousClose),
//            abs(Low-PreviousClose))
//
// Wilder ATR:
//   First ATR = SMA(TR, Period)
//   Next ATR = (PreviousATR*(Period-1) + TR) / Period
//
// Basic Upper Band:
//   (High + Low)/2 + Multiplier*ATR
//
// Basic Lower Band:
//   (High + Low)/2 - Multiplier*ATR
//
// Final Upper Band:
//   BasicUpper if BasicUpper < PreviousUpper
//   OR PreviousClose > PreviousUpper
//   Otherwise PreviousUpper
//
// Final Lower Band:
//   BasicLower if BasicLower > PreviousLower
//   OR PreviousClose < PreviousLower
//   Otherwise PreviousLower
//
// Supertrend:
//   Uptrend   = FinalLowerBand
//   Downtrend = FinalUpperBand
//
// Result:
//   Current, Previous, UpperBand, LowerBand,
//   Direction, IsReady, Timestamp
//
// Note:
// This is a standard Wilder ATR Supertrend variant.
// QuantMan's exact initialization and comparison
// conventions require backtest verification.

type SupertrendConfig struct {
	Period         int
	Multiplier     float64
	CandleInterval string
}

type Supertrend struct {
	id  string
	cfg SupertrendConfig

	lastTimestamp time.Time

	count         int
	previousClose float64

	atrSum   float64
	atr      float64
	atrReady bool

	upper     float64
	lower     float64
	direction int

	hasResult bool
	value     float64

	result SupertrendResult
}

func NewSupertrend(id string, cfg map[string]any) (*Supertrend, error) {
	if id == "" {
		return nil, fmt.Errorf("indicator id is required")
	}

	config := SupertrendConfig{
		Period:         10,
		Multiplier:     2,
		CandleInterval: "1 minutes",
	}

	if value, ok := cfg["period"].(int); ok {
		config.Period = value
	}

	if value, ok := cfg["multiplier"].(float64); ok {
		config.Multiplier = value
	} else if value, ok := cfg["multiplier"].(int); ok {
		config.Multiplier = float64(value)
	}

	if value, ok := cfg["candleInterval"].(string); ok {
		config.CandleInterval = value
	}

	if config.Period <= 0 ||
		config.Multiplier <= 0 ||
		math.IsNaN(config.Multiplier) ||
		math.IsInf(config.Multiplier, 0) {
		return nil, fmt.Errorf("invalid Supertrend configuration")
	}

	return &Supertrend{
		id:  id,
		cfg: config,
	}, nil
}

func (s *Supertrend) Update(candle core.MarketCandle) any {
	s.ingest(candle)
	return s.result
}

func (s *Supertrend) Result() any {
	return s.result
}

func (s *Supertrend) ingest(candle core.MarketCandle) {
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

	// Calculate True Range.
	tr := high - low

	if s.count > 0 {
		tr = math.Max(
			high-low,
			math.Max(
				math.Abs(high-s.previousClose),
				math.Abs(low-s.previousClose),
			),
		)
	}

	previousClose := s.previousClose
	s.previousClose = closePrice
	s.count++

	// Initialize Wilder ATR using first Period TR values.
	if !s.atrReady {
		s.atrSum += tr

		if s.count < period {
			return
		}

		s.atr = s.atrSum / float64(period)
		s.atrReady = true
	} else {
		s.atr = (s.atr*float64(period-1) + tr) / float64(period)
	}

	// Calculate basic bands.
	midPrice := (high + low) / 2.0

	basicUpper := midPrice + s.cfg.Multiplier*s.atr
	basicLower := midPrice - s.cfg.Multiplier*s.atr

	// First ready candle: initialize bands and direction.
	if !s.hasResult {
		s.upper = basicUpper
		s.lower = basicLower

		// Initial trend is set to downtrend.
		// This is a convention, not a QuantMan-confirmed rule.
		s.direction = -1

		s.value = s.upper
		s.hasResult = true

		s.result.Current = s.value
		s.result.Previous = s.value
		s.result.UpperBand = s.upper
		s.result.LowerBand = s.lower
		s.result.Direction = s.direction
		s.result.IsReady = true
		return
	}

	previousUpper := s.upper
	previousLower := s.lower
	previousDirection := s.direction

	// Calculate final upper band.
	finalUpper := basicUpper

	if basicUpper >= previousUpper &&
		previousClose <= previousUpper {
		finalUpper = previousUpper
	}

	// Calculate final lower band.
	finalLower := basicLower

	if basicLower <= previousLower &&
		previousClose >= previousLower {
		finalLower = previousLower
	}

	// Determine trend direction using previous band.
	direction := previousDirection

	if previousDirection == -1 {
		// Downtrend -> Uptrend
		if closePrice > previousUpper {
			direction = 1
		}
	} else {
		// Uptrend -> Downtrend
		if closePrice < previousLower {
			direction = -1
		}
	}

	// Select Supertrend value.
	current := finalLower

	if direction == -1 {
		current = finalUpper
	}

	if math.IsNaN(current) || math.IsInf(current, 0) {
		return
	}

	previous := s.value

	// Update state.
	s.upper = finalUpper
	s.lower = finalLower
	s.direction = direction
	s.value = current

	// Update result.
	s.result.Current = current
	s.result.Previous = previous
	s.result.UpperBand = finalUpper
	s.result.LowerBand = finalLower
	s.result.Direction = direction
	s.result.Timestamp = timestamp
	s.result.IsReady = true
}
