package volatility

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// Indicator Name: Average True Range (ATR)
//
// Pipe Config:
//
//	{
//	  Type: "averageTrueRange",
//	  Name: "averageTrueRange",
//	  Config: map[string]any{
//	    "period":         14,
//	    "candleInterval": "1 minutes",
//	  },
//	}
//
// Entry Condition (example):
//   ATR Is Above Threshold:
//     Current > Threshold
//
//   ATR Crosses Above Threshold:
//     Previous <= Threshold && Current > Threshold
//
// Exit Condition (example):
//   ATR Is Below Threshold:
//     Current < Threshold
//
//   ATR Crosses Below Threshold:
//     Previous >= Threshold && Current < Threshold
//
// Original Formula (Wilder):
//
//   TR = Max(
//       High - Low,
//       Abs(High - PreviousClose),
//       Abs(Low - PreviousClose)
//   )
//
//   FirstATR = Sum(First N TrueRanges) / N
//
//   NextATR =
//       (PreviousATR * (N - 1) + TR) / N
//
// Result:
//   Current, Previous, IsReady, Timestamp
//

type AverageTrueRangeConfig struct {
	Period         int
	CandleInterval string
}

type AverageTrueRange struct {
	id  string
	cfg AverageTrueRangeConfig

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

func NewAverageTrueRange(id string, cfg map[string]any) (*AverageTrueRange, error) {

	if id == "" {
		return nil, fmt.Errorf(
			"averageTrueRange: indicator id is required",
		)
	}

	config := AverageTrueRangeConfig{
		Period:         14,
		CandleInterval: "1 minutes",
	}

	// Read period from pipe config.
	if value, exists := cfg["period"]; exists {
		period, ok := value.(int)

		if !ok {
			return nil, fmt.Errorf(
				"averageTrueRange: period must be an integer",
			)
		}

		config.Period = period
	}

	// Read candle interval from pipe config.
	if value, exists := cfg["candleInterval"]; exists {
		interval, ok := value.(string)

		if !ok || interval == "" {
			return nil, fmt.Errorf(
				"averageTrueRange: invalid candleInterval",
			)
		}

		config.CandleInterval = interval
	}

	if config.Period <= 0 {
		return nil, fmt.Errorf(
			"averageTrueRange: period must be positive",
		)
	}

	return &AverageTrueRange{
		id:  id,
		cfg: config,
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
		// Prevent stale results from being used as new signals.
		a.result.IsReady = false
		return
	}

	high := candle.GetHigh()
	low := candle.GetLow()
	closePrice := candle.GetClose()
	timestamp := candle.GetTime()

	// Step 1: True Range.
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

	a.previousClose = closePrice
	a.hasClose = true
	a.lastTimestamp = timestamp

	// Step 2: First ATR using SMA.
	if a.count < a.cfg.Period {
		a.seedSum += trueRange
		a.count++

		if a.count < a.cfg.Period {
			a.result = AverageTrueRangeResult{
				BaseResult: BaseResult{
					IsReady:   false,
					Timestamp: timestamp,
				},
			}
			return
		}

		a.atr = a.seedSum / float64(a.cfg.Period)

	} else {
		// Step 3: Wilder smoothing.
		a.atr = (a.atr*float64(a.cfg.Period-1) + trueRange) / float64(a.cfg.Period)
	}

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

func validAverageTrueRangeCandle(candle core.MarketCandle, lastTimestamp time.Time) bool {

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
