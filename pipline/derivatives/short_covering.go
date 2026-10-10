package derivatives

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: Short Covering
//
// Short Covering occurs when price increases while
// Open Interest decreases.
//
// This generally indicates existing short positions
// being closed.
//
// pipeConfigs: [
//
//	{
//	  type: 'shortCovering',
//	  name: 'shortCovering',
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
//   Short Covering Is True:
//     Current == true
//
//   Short Covering Becomes True:
//     !Previous && Current
//
// Exit Condition:
//
//   Short Covering Is False:
//     Current == false
//
//   Short Covering Becomes False:
//     Previous && !Current
//
// Original Classification Rule:
//
//   PriceChange = CurrentClose - PreviousClose
//
//   OpenInterestChange = CurrentOI - PreviousOI
//
//   ShortCovering =
//     (PriceChange > 0) && (OpenInterestChange < 0)
//
// BaseResult:
//
//   Current:
//     Latest Short Covering classification.
//
//   Previous:
//     Previous ready Short Covering classification.
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

type ShortCoveringConfig struct {
	CandleInterval string
}

type ShortCoveringResult struct {
	IsReady            bool
	Current            bool
	Previous           bool
	PriceChange        float64
	OpenInterestChange float64
	Timestamp          time.Time
}

type ShortCovering struct {
	id             string
	candleInterval string

	hasClose      bool
	previousClose float64
	previousOI    float64

	lastTimestamp time.Time

	hasResult bool
	lastValue bool

	result ShortCoveringResult
}

func NewShortCovering(id string, cfg ShortCoveringConfig) (*ShortCovering, error) {
	if id == "" {
		return nil, fmt.Errorf("ShortCovering name cannot be empty")
	}

	return &ShortCovering{
		id:             id,
		candleInterval: cfg.CandleInterval,
	}, nil
}

func (r *ShortCovering) Update(candle core.MarketCandle) any {
	r.ingest(candle)
	return r.result
}

func (r *ShortCovering) Result() any {
	return r.result
}

func (r *ShortCovering) ingest(candle core.MarketCandle) {
	if !validShortCoveringCandle(candle, r.lastTimestamp) {
		r.result = ShortCoveringResult{}
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

		r.result = ShortCoveringResult{
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

	// Short Covering classification.
	current := priceChange > 0 && openInterestChange < 0

	previous := current
	if r.hasResult {
		previous = r.lastValue
	}

	r.result = ShortCoveringResult{
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

func validShortCoveringCandle(candle core.MarketCandle, lastTimestamp time.Time) bool {

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
