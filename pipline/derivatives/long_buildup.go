package derivatives

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: Long Buildup
//
// Long Buildup occurs when both price and Open Interest increase.
// It generally indicates fresh long positions entering the market.
//
// pipeConfigs: [
//
//	{
//	  type: 'longBuildup',
//	  name: 'longBuildup',
//	  config: {
//	    candleInterval: '1 minutes',
//	  },
//	}
//
// ],
//
// Available result:
//
//   - Long Buildup
//   - Current
//   - Previous
//   - PriceChange
//   - OpenInterestChange
//   - IsReady
//   - Timestamp
//
// Entry Condition:
//
//   Long Buildup Is True
//   Current == true
//
//   Long Buildup Becomes True
//   Previous == false && Current == true
//
// Exit Condition:
//
//   Long Buildup Is False
//   Current == false
//
//   Long Buildup Becomes False
//   Previous == true && Current == false
//
// Original Classification Rule:
//
//   PriceChange = CurrentClose - PreviousClose
//
//   OpenInterestChange = CurrentOI - PreviousOI
//
//   Long Buildup =
//     (PriceChange > 0) && (OpenInterestChange > 0)
//
// Explanation:
//
//   Current represents the latest Long Buildup classification.
//
//   Previous represents the preceding ready classification.
//
//   PriceChange represents the difference between
//   the current close and previous close.
//
//   OpenInterestChange represents the difference between
//   the current and previous Open Interest.
//
//   IsReady becomes true after two valid candles.
//
//   Entry and Exit conditions are examples.
//   Strategy logic determines the actual trading conditions.
//
// Reference:
// https://www.nseindia.com/learn/overview-about-derivatives
//
// Note:
// This uses the standard price/OI classification.
// QuantMan exact calculation parity is not yet verified.

type LongBuildupConfig struct {
	CandleInterval string
}

type LongBuildupResult struct {
	IsReady            bool
	Current            bool
	Previous           bool
	PriceChange        float64
	OpenInterestChange float64
	Timestamp          time.Time
}

type LongBuildup struct {
	id             string
	candleInterval string

	previousClose float64
	previousOI    float64
	hasClose      bool

	hasResult bool
	lastValue bool

	lastTimestamp time.Time

	result LongBuildupResult
}

func NewLongBuildup(id string, cfg LongBuildupConfig) (*LongBuildup, error) {
	if id == "" {
		return nil, fmt.Errorf("Long Buildup name cannot be empty")
	}

	return &LongBuildup{
		id:             id,
		candleInterval: cfg.CandleInterval,
	}, nil
}

func (r *LongBuildup) Update(candle core.MarketCandle) any {
	r.ingest(candle)
	return r.result
}

func (r *LongBuildup) Result() any {
	return r.result
}

func (r *LongBuildup) ingest(candle core.MarketCandle) {
	if !validLongBuildupCandle(candle) {
		r.result = LongBuildupResult{}
		return
	}

	closePrice := candle.GetClose()
	openInterest := candle.GetOpenInterest()
	timestamp := candle.GetTime()

	// Reject duplicate or out-of-order candles.
	if !r.lastTimestamp.IsZero() &&
		!timestamp.After(r.lastTimestamp) {

		r.result = LongBuildupResult{
			IsReady:   false,
			Timestamp: timestamp,
		}
		return
	}

	r.lastTimestamp = timestamp

	// First candle provides reference values.
	if !r.hasClose {
		r.previousClose = closePrice
		r.previousOI = openInterest
		r.hasClose = true

		r.result = LongBuildupResult{
			IsReady:   false,
			Timestamp: timestamp,
		}
		return
	}

	// Calculate price and Open Interest changes.
	priceChange := closePrice - r.previousClose
	openInterestChange := openInterest - r.previousOI

	r.previousClose = closePrice
	r.previousOI = openInterest

	// Long Buildup classification.
	current := priceChange > 0 && openInterestChange > 0

	previous := current
	if r.hasResult {
		previous = r.lastValue
	}

	r.result = LongBuildupResult{
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

func validLongBuildupCandle(candle core.MarketCandle) bool {
	if candle == nil || candle.GetTime().IsZero() {
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
