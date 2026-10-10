package volatility

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: Average True Range (ATR) - Wilder's volatility indicator
//
// pipeConfigs: [
//
//	{
//	  type: 'averageTrueRange',
//	  name: 'averageTrueRange',
//	  config: {
//	    noOfCandles: 14,
//	    candleInterval: '1 minutes',
//	  }
//	}
//
// ],
//
// Available result:
//
//   - Average True Range (Current, Previous, IsReady, Timestamp)
//
// Entry Condition:
//
//	ATR Is Above Threshold:
//	  Current > Threshold
//
//	ATR Crosses Above Threshold:
//	  Previous <= Threshold && Current > Threshold
//
// Exit Condition:
//
//	ATR Is Below Threshold:
//	  Current < Threshold
//
//	ATR Crosses Below Threshold:
//	  Previous >= Threshold && Current < Threshold
//
// Original Formula (Wilder):
//
//	TrueRange = Max(
//	  High - Low,
//	  Abs(High - PreviousClose),
//	  Abs(Low - PreviousClose)
//	)
//
//	FirstATR = Sum(First N TrueRanges) / N
//
//	NextATR = ((PreviousATR * (N - 1)) + TrueRange) / N
//
// BaseResult:
//
//	Current:
//	  Latest ATR value.
//
//	Previous:
//	  Previous ready ATR value.
//
//	IsReady:
//	  True after N valid candles.
//
//	Timestamp:
//	  Timestamp of the latest processed valid candle.
//
// Warmup:
//
//	The first candle's TrueRange is High - Low.
//	The first ready result requires N valid candles.
//	Previous equals Current on the first ready candle.
//
// Reference:
// https://chartschool.stockcharts.com/table-of-contents/technical-indicators-and-overlays/technical-indicators/average-true-range-atr
//
// Note:
// QuantMan exact calculation parity has not been verified.

type AverageTrueRangeConfig struct {
	NoOfCandles    int
	CandleInterval string
}

type AverageTrueRange struct {
	id             string
	period         int
	candleInterval string

	previousClose float64
	hasClose      bool

	count   int
	seedSum float64
	atr     float64

	hasResult bool
	lastValue float64

	lastTimestamp time.Time

	result AverageTrueRangeResult
}

func NewAverageTrueRange(id string, cfg AverageTrueRangeConfig) (*AverageTrueRange, error) {
	if id == "" {
		return nil, fmt.Errorf("ATR name cannot be empty")
	}

	if cfg.NoOfCandles <= 0 {
		return nil, fmt.Errorf(
			"ATR period must be positive: %d",
			cfg.NoOfCandles,
		)
	}

	return &AverageTrueRange{
		id:             id,
		period:         cfg.NoOfCandles,
		candleInterval: cfg.CandleInterval,
	}, nil
}

func (a *AverageTrueRange) Update(candle core.MarketCandle) any {
	a.ingest(candle)
	return a.result
}

func (a *AverageTrueRange) Result() any {
	return a.result
}

func (a *AverageTrueRange) ingest(candle core.MarketCandle) {
	
	if !validAverageTrueRangeCandle(candle, a.lastTimestamp) {
		return
	}

	high := candle.GetHigh()
	low := candle.GetLow()
	closePrice := candle.GetClose()
	timestamp := candle.GetTime()

	// Calculate True Range.
	trueRange := high - low

	if a.hasClose {
		trueRange = math.Max(
			trueRange,
			math.Max(
				math.Abs(high-a.previousClose),
				math.Abs(low-a.previousClose),
			),
		)
	}

	// Update previous close and timestamp.
	a.previousClose = closePrice
	a.hasClose = true
	a.lastTimestamp = timestamp

	// Initial ATR calculation.
	if a.count < a.period {
		a.seedSum += trueRange
		a.count++

		// Wait until N valid candles.
		if a.count < a.period {
			a.result = AverageTrueRangeResult{
				BaseResult: BaseResult{
					IsReady:   false,
					Timestamp: timestamp,
				},
			}
			return
		}

		// First ATR = Arithmetic mean of first N true ranges.
		a.atr = a.seedSum / float64(a.period)
	} else {
		// Wilder smoothing.
		a.atr = (a.atr*float64(a.period-1) + trueRange) / float64(a.period)
	}

	// Previous equals Current on first ready candle.
	previous := a.atr

	if a.hasResult {
		previous = a.lastValue
	}

	a.result = AverageTrueRangeResult{
		BaseResult: BaseResult{
			IsReady:   true,
			Current:   a.atr,
			Previous:  previous,
			Timestamp: timestamp,
		},
	}

	a.lastValue = a.atr
	a.hasResult = true
}

// Own helper function for Average True Range.

func validAverageTrueRangeCandle(candle core.MarketCandle, lastTimestamp time.Time) bool {

	if candle == nil {
		return false
	}

	timestamp := candle.GetTime()

	if timestamp.IsZero() {
		return false
	}

	// Reject duplicate and out-of-order candles.
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
