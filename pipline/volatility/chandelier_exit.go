package volatility

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: Chandelier Exit
//
// Chandelier Exit is an ATR-based trailing stop indicator.
//
// pipeConfigs: [
//   {
//     type: 'chandelierExit',
//     name: 'chandelierExit',
//     config: {
//       period: 22,
//       multiplier: 3,
//       candleInterval: '1 minutes'
//     }
//   }
// ]
//
// Available result:
//   - Current: Long Exit
//   - Previous: Previous ready Long Exit
//   - IsReady
//   - Timestamp
//
// Entry Condition:
//   Is Above: Current > threshold
//   Crosses Above: Previous <= threshold && Current > threshold
//
// Exit Condition:
//   Is Below: Current < threshold
//   Crosses Below: Previous >= threshold && Current < threshold
//
// Original Formula:
//
//   True Range = max(
//     High - Low,
//     abs(High - PreviousClose),
//     abs(Low - PreviousClose)
//   )
//
//   ATR Seed = Sum(first Period true ranges) / Period
//
//   Wilder ATR =
//     (PreviousATR * (Period - 1) + CurrentTR) / Period
//
//   Long Exit = HighestHigh(Period) - Multiplier * ATR
//
//   Short Exit = LowestLow(Period) + Multiplier * ATR
//
// Result:
//   Current and Previous expose the Long Exit only.
//   Short Exit is not exposed by ScalarResult.
//
// Warmup:
//   The first candle establishes PreviousClose.
//   The next Period candles seed ATR.
//   First ready result requires Period + 1 valid candles.
//
// Reference:
// https://chartschool.stockcharts.com/table-of-contents/technical-indicators-and-overlays/technical-overlays/chandelier-exit
//
// Note:
// QuantMan exact parity is not verified.

type ChandelierExitConfig struct {
	Period         int
	Multiplier     float64
	CandleInterval string
}

type ChandelierExit struct {
	id             string
	period         int
	multiplier     float64
	candleInterval string

	highs []float64
	lows  []float64

	atr     float64
	trSum   float64
	trCount int

	lastClose float64
	hasClose  bool

	lastTime time.Time

	hasResult bool
	result    ScalarResult
}

func NewChandelierExit(id string, cfg ChandelierExitConfig) (*ChandelierExit, error) {
	if id == "" {
		return nil, fmt.Errorf("Chandelier Exit name cannot be empty")
	}

	if cfg.Period < 1 {
		return nil, fmt.Errorf("Chandelier Exit period must be greater than zero")
	}

	if cfg.Multiplier <= 0 ||
		math.IsNaN(cfg.Multiplier) ||
		math.IsInf(cfg.Multiplier, 0) {
		return nil, fmt.Errorf("Chandelier Exit multiplier must be finite and greater than zero")
	}

	return &ChandelierExit{
		id:             id,
		period:         cfg.Period,
		multiplier:     cfg.Multiplier,
		candleInterval: cfg.CandleInterval,
	}, nil
}

func (r *ChandelierExit) Update(candle core.MarketCandle) any {
	r.ingest(candle)
	return r.result
}

func (r *ChandelierExit) Result() any {
	return r.result
}

func (r *ChandelierExit) ingest(candle core.MarketCandle) {
	if !validChandelierExitCandle(candle, r.lastTime) {
		return
	}

	timestamp := candle.GetTime()
	high := candle.GetHigh()
	low := candle.GetLow()
	closePrice := candle.GetClose()

	// First candle establishes previous close.
	// No True Range is accumulated yet.
	if !r.hasClose {
		r.lastClose = closePrice
		r.hasClose = true
		r.lastTime = timestamp

		r.highs = append(r.highs, high)
		r.lows = append(r.lows, low)

		r.result = ScalarResult{
			IsReady:   false,
			Timestamp: timestamp,
		}
		return
	}

	// Calculate True Range using previous close.
	trueRange := math.Max(
		high-low,
		math.Max(
			math.Abs(high-r.lastClose),
			math.Abs(low-r.lastClose),
		),
	)

	r.lastClose = closePrice
	r.lastTime = timestamp

	// Maintain rolling highs and lows.
	r.highs = append(r.highs, high)
	r.lows = append(r.lows, low)

	if len(r.highs) > r.period {
		r.highs = r.highs[1:]
		r.lows = r.lows[1:]
	}

	// Seed ATR using the first Period true ranges.
	if r.trCount < r.period {
		r.trSum += trueRange
		r.trCount++

		if r.trCount == r.period {
			r.atr = r.trSum / float64(r.period)
		}
	} else {
		// Wilder smoothing.
		r.atr = (r.atr*float64(r.period-1) + trueRange) / float64(r.period)
	}

	// Wait until ATR and rolling highs are ready.
	if r.trCount < r.period ||
		len(r.highs) < r.period {
		r.result = ScalarResult{
			IsReady:   false,
			Timestamp: timestamp,
		}
		return
	}

	highestHigh := r.highs[0]

	for _, value := range r.highs[1:] {
		if value > highestHigh {
			highestHigh = value
		}
	}

	// Chandelier Long Exit.
	current := highestHigh - r.multiplier*r.atr

	if math.IsNaN(current) || math.IsInf(current, 0) {
		return
	}

	previous := current

	if r.hasResult {
		previous = r.result.Current
	}

	r.result = ScalarResult{
		IsReady:   true,
		Current:   current,
		Previous:  previous,
		Timestamp: timestamp,
	}

	r.hasResult = true
}

// Own validation helper for Chandelier Exit.

func validChandelierExitCandle(candle core.MarketCandle, lastTimestamp time.Time) bool {

	if candle == nil {
		return false
	}

	timestamp := candle.GetTime()

	if timestamp.IsZero() {
		return false
	}

	if !lastTimestamp.IsZero() &&
		!timestamp.After(lastTimestamp) {
		return false
	}

	open := candle.GetOpen()
	high := candle.GetHigh()
	low := candle.GetLow()
	closePrice := candle.GetClose()

	values := []float64{
		open,
		high,
		low,
		closePrice,
	}

	for _, value := range values {
		if math.IsNaN(value) ||
			math.IsInf(value, 0) ||
			value <= 0 {
			return false
		}
	}

	if high < low {
		return false
	}

	if high < math.Max(open, closePrice) {
		return false
	}

	if low > math.Min(open, closePrice) {
		return false
	}

	return true
}
