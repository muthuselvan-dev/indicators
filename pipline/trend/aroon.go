package trend

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// Aroon

//{
//     Type: "aroon",
//     Name: "aroon",
//     Config: map[string]any{
//         "period":          14,
//         "candleInterval": "1 minutes",
//     },
// },

// Entry : Aroon Up is above Aroon Down
// Exit : Aroon Up is below Aroon Down

// Formula:
// Aroon Up   = 100 * (Period - 1 - PeriodsSinceHighestHigh) / (Period - 1)
// Aroon Down = 100 * (Period - 1 - PeriodsSinceLowestLow) / (Period - 1)
//
// Current = Aroon Up - Aroon Down
//
// Result:
// Up, Down, Current, Previous, IsReady, Timestamp

type Aroon struct {
	id             string
	period         int
	candleInterval string

	closes []float64
	highs  []float64
	lows   []float64

	lastTimestamp time.Time

	hasResult bool
	value     float64

	result AroonResult
}

func NewAroon(id string, cfg map[string]any) (*Aroon, error) {
	if id == "" {
		return nil, fmt.Errorf("indicator id is required")
	}

	period := 14
	candleInterval := "1 minutes"

	if value, ok := cfg["noOfCandles"].(int); ok {
		period = value
	}

	if value, ok := cfg["period"].(int); ok {
		period = value
	}

	if value, ok := cfg["candleInterval"].(string); ok {
		candleInterval = value
	}

	return &Aroon{
		id:             id,
		period:         period,
		candleInterval: candleInterval,
	}, nil
}

func (s *Aroon) Update(candle core.MarketCandle) any {
	s.ingest(candle)
	return s.result
}

func (s *Aroon) Result() any {
	return s.result
}

func (s *Aroon) ingest(candle core.MarketCandle) {
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

		s.result.IsReady = false
		return
	}

	s.lastTimestamp = timestamp

	// Store candle data.
	s.closes = append(s.closes, closePrice)
	s.highs = append(s.highs, high)
	s.lows = append(s.lows, low)

	period := s.period

	// Preserve existing history limit.
	maxHistory := max(500, 4*period+10)

	if len(s.closes) > maxHistory {
		s.closes = s.closes[1:]
		s.highs = s.highs[1:]
		s.lows = s.lows[1:]
	}

	// Warm-up.
	if len(s.closes) < period {
		s.result.IsReady = false
		s.result.Timestamp = timestamp
		return
	}

	// Find highest high and lowest low.
	start := len(s.closes) - period

	maxIdx := start
	minIdx := start

	for i := start + 1; i < len(s.closes); i++ {
		if s.highs[i] >= s.highs[maxIdx] {
			maxIdx = i
		}

		if s.lows[i] <= s.lows[minIdx] {
			minIdx = i
		}
	}

	// Periods since the latest extreme.
	periodsSinceHigh := len(s.closes) - 1 - maxIdx
	periodsSinceLow := len(s.closes) - 1 - minIdx

	// Preserve existing Aroon formula.
	up := 100 *
		float64(period-1-periodsSinceHigh) /
		float64(period-1)

	down := 100 *
		float64(period-1-periodsSinceLow) /
		float64(period-1)

	current := up - down

	if math.IsNaN(current) || math.IsInf(current, 0) {
		s.result.IsReady = false
		return
	}

	previous := current

	if s.hasResult {
		previous = s.value
	}

	s.result.Up = up
	s.result.Down = down
	s.result.Current = current
	s.result.Previous = previous
	s.result.IsReady = true
	s.result.Timestamp = timestamp

	s.value = current
	s.hasResult = true
}
