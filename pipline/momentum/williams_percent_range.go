package momentum

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: Williams Percent Range (Williams %R)
//
// pipeConfigs: [
//
//	{
//	  type: 'williamsPercentRange',
//	  name: 'williamsR',
//	  config: {
//	    period: 14,
//	    candleInterval: '1 minutes',
//	  }
//	}
//
// ],
//
// Available result:
//
//   - Williams Percent Range (%R)
//
// Entry Condition:
//
//   Williams %R Crosses Above -80
//   Previous <= -80 && Current > -80
//
// Exit Condition:
//
//   Williams %R Crosses Below -20
//   Previous >= -20 && Current < -20
//
// Original Formula:
//
// HighestHigh = Highest High over period candles
// LowestLow = Lowest Low over period candles
//
// Williams %R =
//   -100 * (HighestHigh - Close) /
//          (HighestHigh - LowestLow)
//
// Equivalent Formula:
//
// Williams %R =
//   100 * (Close - HighestHigh) /
//         (HighestHigh - LowestLow)
//
// Range: -100 to 0
//
// -80 to -100 = Oversold zone
// -20 to 0    = Overbought zone
//
// When HighestHigh == LowestLow:
//   Williams %R = -50 (neutral fallback)
//
// Current represents current Williams %R.
// Previous represents previous Williams %R.
//
// With period 14, the first Williams %R
// is ready after 14 valid candles.

type WilliamsPercentRangeConfig struct {
	Period         int
	CandleInterval string
}

type WilliamsPercentRange struct {
	id string

	period         int
	candleInterval string

	highs []float64
	lows  []float64

	hasResult bool
	lastValue float64

	lastTimestamp time.Time

	result WilliamsPercentRangeResult
}

func NewWilliamsPercentRange(id string, cfg WilliamsPercentRangeConfig) (*WilliamsPercentRange, error) {
	if id == "" {
		return nil, fmt.Errorf("Williams Percent Range name cannot be empty")
	}

	if cfg.Period <= 0 {
		return nil, fmt.Errorf("Williams Percent Range period must be positive: %d", cfg.Period)
	}

	return &WilliamsPercentRange{
		id:             id,
		period:         cfg.Period,
		candleInterval: cfg.CandleInterval,
	}, nil
}

func (w *WilliamsPercentRange) Update(candle core.MarketCandle) any {
	w.ingest(candle)
	return w.result
}

func (w *WilliamsPercentRange) Result() any {
	return w.result
}

func (w *WilliamsPercentRange) ingest(candle core.MarketCandle) {
	if candle == nil {
		return
	}

	timestamp := candle.GetTime()
	high := candle.GetHigh()
	low := candle.GetLow()
	closePrice := candle.GetClose()

	// Reject invalid or out-of-order candles.
	if timestamp.IsZero() ||
		!validWilliamsNumber(high) ||
		!validWilliamsNumber(low) ||
		!validWilliamsNumber(closePrice) ||
		low <= 0 ||
		high < low ||
		closePrice < low ||
		closePrice > high ||
		(!w.lastTimestamp.IsZero() &&
			!timestamp.After(w.lastTimestamp)) {

		w.setNotReady(timestamp)
		return
	}

	w.lastTimestamp = timestamp

	// Maintain rolling high and low windows.
	w.highs = append(w.highs, high)
	w.lows = append(w.lows, low)

	if len(w.highs) > w.period {
		w.highs = w.highs[1:]
		w.lows = w.lows[1:]
	}

	// Wait until the configured period is available.
	if len(w.highs) < w.period {
		w.setNotReady(timestamp)
		return
	}

	highestHigh := w.highs[0]
	lowestLow := w.lows[0]

	// Find the highest high and lowest low.
	for i := 1; i < len(w.highs); i++ {
		if w.highs[i] > highestHigh {
			highestHigh = w.highs[i]
		}

		if w.lows[i] < lowestLow {
			lowestLow = w.lows[i]
		}
	}

	// Calculate Williams %R.
	currentWilliamsR := -50.0

	if highestHigh != lowestLow {
		currentWilliamsR = -100 * (highestHigh - closePrice) / (highestHigh - lowestLow)
	}

	if !validWilliamsNumber(currentWilliamsR) {
		w.setNotReady(timestamp)
		return
	}

	previousWilliamsR := currentWilliamsR

	if w.hasResult {
		previousWilliamsR = w.lastValue
	}

	w.result = WilliamsPercentRangeResult{
		IsReady:   true,
		Current:   currentWilliamsR,
		Previous:  previousWilliamsR,
		Timestamp: timestamp,
	}

	w.lastValue = currentWilliamsR
	w.hasResult = true
}

func (w *WilliamsPercentRange) setNotReady(timestamp time.Time) {
	w.result = WilliamsPercentRangeResult{
		IsReady:   false,
		Timestamp: timestamp,
	}
}

func validWilliamsNumber(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
