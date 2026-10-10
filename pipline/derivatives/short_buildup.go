package derivatives

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: Short Buildup
//
// Short Buildup occurs when price decreases while
// Open Interest increases.
//
// This generally indicates fresh short positions
// entering the market.
//
// pipeConfigs: [
//
//	{
//	  type: 'shortBuildup',
//	  name: 'shortBuildup',
//	  config: {
//	    candleInterval: '1 minutes',
//	  },
//	}
//
// ]
//
// Available result:
//
//   - Current
//   - Previous
//   - PriceChange
//   - OpenInterestChange
//   - IsReady
//   - Timestamp
//
// Entry Condition:
//
//   Short Buildup Is True:
//     Current == true
//
//   Short Buildup Becomes True:
//     !Previous && Current
//
// Exit Condition:
//
//   Short Buildup Is False:
//     Current == false
//
//   Short Buildup Becomes False:
//     Previous && !Current
//
// Original Classification Rule:
//
//   PriceChange = CurrentClose - PreviousClose
//
//   OpenInterestChange = CurrentOI - PreviousOI
//
//   ShortBuildup =
//     (PriceChange < 0) && (OpenInterestChange > 0)
//
// BaseResult:
//
//   Current:
//     Latest Short Buildup classification.
//
//   Previous:
//     Previous ready Short Buildup classification.
//
//   PriceChange:
//     Current close minus previous close.
//
//   OpenInterestChange:
//     Current OI minus previous OI.
//
// Warmup:
//
//   First valid candle stores reference values.
//   Second valid candle produces first ready result.
//
// Reference:
// https://www.nseindia.com/learn/overview-about-derivatives
//
// Note:
// QuantMan exact classification parity is not verified.

type ShortBuildupConfig struct {
	CandleInterval string
}

type ShortBuildupResult struct {
	IsReady            bool
	Current            bool
	Previous           bool
	PriceChange        float64
	OpenInterestChange float64
	Timestamp          time.Time
}

type ShortBuildup struct {
	id             string
	candleInterval string

	hasClose      bool
	previousClose float64
	previousOI    float64

	lastTimestamp time.Time

	hasResult bool
	lastValue bool

	result ShortBuildupResult
}

func NewShortBuildup(id string, cfg ShortBuildupConfig) (*ShortBuildup, error) {
	if id == "" {
		return nil, fmt.Errorf("ShortBuildup name cannot be empty")
	}

	return &ShortBuildup{
		id:             id,
		candleInterval: cfg.CandleInterval,
	}, nil
}

func (r *ShortBuildup) Update(candle core.MarketCandle) any {
	r.ingest(candle)
	return r.result
}

func (r *ShortBuildup) Result() any {
	return r.result
}

func (r *ShortBuildup) ingest(candle core.MarketCandle) {
	if !validShortBuildupCandle(candle, r.lastTimestamp) {
		r.result = ShortBuildupResult{}
		return
	}

	closePrice := candle.GetClose()
	openInterest := candle.GetOpenInterest()
	timestamp := candle.GetTime()

	// First valid candle stores reference values.
	if !r.hasClose {
		r.previousClose = closePrice
		r.previousOI = openInterest
		r.lastTimestamp = timestamp
		r.hasClose = true

		r.result = ShortBuildupResult{
			IsReady:   false,
			Timestamp: timestamp,
		}
		return
	}

	// Calculate price and Open Interest changes.
	priceChange := closePrice - r.previousClose
	openInterestChange := openInterest - r.previousOI

	// Update reference values.
	r.previousClose = closePrice
	r.previousOI = openInterest
	r.lastTimestamp = timestamp

	// Short Buildup classification.
	current := priceChange < 0 && openInterestChange > 0

	previous := current
	if r.hasResult {
		previous = r.lastValue
	}

	r.result = ShortBuildupResult{
		IsReady:            true,
		Current:            current,
		Previous:           previous,
		PriceChange:        priceChange,
		OpenInterestChange: openInterestChange,
		Timestamp:          timestamp,
	}

	r.lastValue = current
	r.hasResult = true
}

func validShortBuildupCandle(
	candle core.MarketCandle,
	lastTimestamp time.Time,
) bool {
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

	closePrice := candle.GetClose()
	openInterest := candle.GetOpenInterest()

	if math.IsNaN(closePrice) ||
		math.IsInf(closePrice, 0) ||
		closePrice <= 0 {
		return false
	}

	if math.IsNaN(openInterest) ||
		math.IsInf(openInterest, 0) ||
		openInterest < 0 {
		return false
	}

	return true
}
