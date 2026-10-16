package trend

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// Parabolic SAR
//
// QuantMan pipeConfig:
//
// type: "parabolicSAR"
// name: "parabolicSAR"
//
// config:
//   start:          0.02
//   increment:      0.02
//   max:            0.20
//   candleInterval: "1 minutes"
//
// Formula:
//
// SAR = PreviousSAR + AF * (EP - PreviousSAR)
//
// AF = Acceleration Factor
// EP = Extreme Point
//
// Direction:
//    1 = Uptrend
//   -1 = Downtrend
//
// Result:
//   Current
//   Previous
//   Direction
//   IsReady
//   Timestamp

type ParabolicSARConfig struct {
	Start          float64
	Increment      float64
	Max            float64
	CandleInterval string
}

type ParabolicSAR struct {
	id  string
	cfg ParabolicSARConfig

	lastTimestamp time.Time

	count int

	previousHigh  float64
	previousLow   float64
	previousClose float64

	olderHigh float64
	olderLow  float64

	direction int

	sar     float64
	extreme float64
	accel   float64

	hasResult bool
	value     float64

	result ParabolicSARResult
}

func NewParabolicSAR(
	id string,
	cfg map[string]any,
) (*ParabolicSAR, error) {
	if id == "" {
		return nil, fmt.Errorf("indicator id is required")
	}

	config := ParabolicSARConfig{
		Start:          0.02,
		Increment:      0.02,
		Max:            0.20,
		CandleInterval: "1 minutes",
	}

	if v, ok := cfg["start"].(float64); ok {
		config.Start = v
	} else if v, ok := cfg["start"].(int); ok {
		config.Start = float64(v)
	}

	if v, ok := cfg["increment"].(float64); ok {
		config.Increment = v
	} else if v, ok := cfg["increment"].(int); ok {
		config.Increment = float64(v)
	}

	if v, ok := cfg["max"].(float64); ok {
		config.Max = v
	} else if v, ok := cfg["max"].(int); ok {
		config.Max = float64(v)
	}

	if v, ok := cfg["candleInterval"].(string); ok {
		config.CandleInterval = v
	}

	if config.Start <= 0 ||
		config.Increment <= 0 ||
		config.Max < config.Start ||
		math.IsNaN(config.Start) ||
		math.IsNaN(config.Increment) ||
		math.IsNaN(config.Max) ||
		math.IsInf(config.Start, 0) ||
		math.IsInf(config.Increment, 0) ||
		math.IsInf(config.Max, 0) {
		return nil, fmt.Errorf("invalid Parabolic SAR configuration")
	}

	return &ParabolicSAR{
		id:  id,
		cfg: config,
	}, nil
}

func (s *ParabolicSAR) Update(
	candle core.MarketCandle,
) any {
	s.ingest(candle)
	return s.result
}

func (s *ParabolicSAR) Result() any {
	return s.result
}

func (s *ParabolicSAR) ingest(
	candle core.MarketCandle,
) {
	s.result.IsReady = false

	if candle == nil {
		return
	}

	timestamp := candle.GetTime()
	high := candle.GetHigh()
	low := candle.GetLow()
	closePrice := candle.GetClose()

	// Validate candle.
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

	// First candle: store initial OHLC.
	if s.count == 0 {
		s.previousHigh = high
		s.previousLow = low
		s.previousClose = closePrice
		s.count = 1
		return
	}

	// Second candle: initialize trend, SAR and EP.
	if s.count == 1 {
		if closePrice >= s.previousClose {
			s.direction = 1
			s.sar = s.previousLow
			s.extreme = math.Max(
				s.previousHigh,
				high,
			)
		} else {
			s.direction = -1
			s.sar = s.previousHigh
			s.extreme = math.Min(
				s.previousLow,
				low,
			)
		}

		s.accel = s.cfg.Start

		s.olderHigh = s.previousHigh
		s.olderLow = s.previousLow

		s.previousHigh = high
		s.previousLow = low
		s.previousClose = closePrice

		s.count = 2

		s.value = s.sar
		s.hasResult = true

		s.result.Current = s.sar
		s.result.Previous = s.sar
		s.result.Direction = s.direction
		s.result.IsReady = true
		return
	}

	// Original Parabolic SAR calculation.
	current := s.sar +
		s.accel*(s.extreme-s.sar)

	if s.direction == 1 {
		// Uptrend: SAR must not exceed
		// either of the previous two lows.
		current = math.Min(
			current,
			s.previousLow,
		)

		current = math.Min(
			current,
			s.olderLow,
		)

		// Bearish reversal.
		if low < current {
			s.direction = -1
			current = s.extreme
			s.extreme = low
			s.accel = s.cfg.Start
		} else if high > s.extreme {
			// New high: increase AF.
			s.extreme = high
			s.accel = math.Min(
				s.cfg.Max,
				s.accel+s.cfg.Increment,
			)
		}
	} else {
		// Downtrend: SAR must not be below
		// either of the previous two highs.
		current = math.Max(
			current,
			s.previousHigh,
		)

		current = math.Max(
			current,
			s.olderHigh,
		)

		// Bullish reversal.
		if high > current {
			s.direction = 1
			current = s.extreme
			s.extreme = high
			s.accel = s.cfg.Start
		} else if low < s.extreme {
			// New low: increase AF.
			s.extreme = low
			s.accel = math.Min(
				s.cfg.Max,
				s.accel+s.cfg.Increment,
			)
		}
	}

	previous := current

	if s.hasResult {
		previous = s.value
	}

	// Update indicator state.
	s.sar = current
	s.value = current
	s.hasResult = true

	s.result.Current = current
	s.result.Previous = previous
	s.result.Direction = s.direction
	s.result.Timestamp = timestamp
	s.result.IsReady = true

	// Shift previous candle history.
	s.olderHigh = s.previousHigh
	s.olderLow = s.previousLow

	s.previousHigh = high
	s.previousLow = low
	s.previousClose = closePrice

	s.count++
}
