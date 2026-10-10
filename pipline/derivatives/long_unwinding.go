package derivatives

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: Long Unwinding
//
// Long Unwinding occurs when price decreases and
// Open Interest also decreases.
//
// This generally indicates existing long positions
// being closed.
//
// pipeConfigs: [
//
//	{
//	  type: 'longUnwinding',
//	  name: 'longUnwinding',
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
//   Long Unwinding Is True:
//     Current == true
//
//   Long Unwinding Becomes True:
//     !Previous && Current
//
// Exit Condition:
//
//   Long Unwinding Is False:
//     Current == false
//
//   Long Unwinding Becomes False:
//     Previous && !Current
//
// Original Classification Rule:
//
//   PriceChange = CurrentClose - PreviousClose
//
//   OpenInterestChange = CurrentOI - PreviousOI
//
//   LongUnwinding =
//     (PriceChange < 0) && (OpenInterestChange < 0)
//
// BaseResult:
//
//   Current:
//     Latest Long Unwinding classification.
//
//   Previous:
//     Previous ready Long Unwinding classification.
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

type LongUnwindingConfig struct {
	CandleInterval string
}

type LongUnwindingResult struct {
	IsReady            bool
	Current            bool
	Previous           bool
	PriceChange        float64
	OpenInterestChange float64
	Timestamp          time.Time
}

type LongUnwinding struct {
	id             string
	candleInterval string

	hasClose      bool
	previousClose float64
	previousOI    float64

	lastTimestamp time.Time

	hasResult bool
	lastValue bool

	result LongUnwindingResult
}

func NewLongUnwinding(id string, cfg LongUnwindingConfig) (*LongUnwinding, error) {
	if id == "" {
		return nil, fmt.Errorf("LongUnwinding name cannot be empty")
	}

	return &LongUnwinding{
		id:             id,
		candleInterval: cfg.CandleInterval,
	}, nil
}

func (r *LongUnwinding) Update(candle core.MarketCandle) any {
	r.ingest(candle)
	return r.result
}

func (r *LongUnwinding) Result() any {
	return r.result
}

func (r *LongUnwinding) ingest(candle core.MarketCandle) {
	if !validLongUnwindingCandle(candle, r.lastTimestamp) {
		r.result = LongUnwindingResult{}
		return
	}

	closePrice := candle.GetClose()
	openInterest := candle.GetOpenInterest()
	timestamp := candle.GetTime()

	// First valid candle is used as the reference.
	if !r.hasClose {
		r.previousClose = closePrice
		r.previousOI = openInterest
		r.lastTimestamp = timestamp
		r.hasClose = true

		r.result = LongUnwindingResult{
			IsReady:   false,
			Timestamp: timestamp,
		}
		return
	}

	// Calculate changes.
	priceChange := closePrice - r.previousClose
	openInterestChange := openInterest - r.previousOI

	// Update reference values.
	r.previousClose = closePrice
	r.previousOI = openInterest
	r.lastTimestamp = timestamp

	// Long Unwinding classification.
	current := priceChange < 0 && openInterestChange < 0

	previous := current
	if r.hasResult {
		previous = r.lastValue
	}

	r.result = LongUnwindingResult{
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

func validLongUnwindingCandle(candle core.MarketCandle, lastTimestamp time.Time) bool {

	if candle == nil {
		return false
	}

	timestamp := candle.GetTime()

	if timestamp.IsZero() {
		return false
	}

	// Reject duplicate or older candles.
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
