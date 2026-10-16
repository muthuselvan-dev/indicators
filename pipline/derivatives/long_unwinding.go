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
// pipeConfigs: [
//   {
//     type: 'longUnwidening',
//     name: 'longUnwinding',
//     config: {
//       candleInterval: '1 minutes',
//     },
//   }
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
//     Previous == false && Current == true
//
// Exit Condition:
//
//   Long Unwinding Is False:
//     Current == false
//
//   Long Unwinding Becomes False:
//     Previous == true && Current == false
//
// Formula:
//
// PriceChange = CurrentClose - PreviousClose
//
// OpenInterestChange = CurrentOI - PreviousOI
//
// LongUnwinding =
//   (PriceChange < 0) && (OpenInterestChange < 0)
//
// Warmup:
//
// First valid candle stores reference values.
// Second valid candle produces first ready result.
//
// Note:
// QuantMan exact classification parity is not verified.

type LongUnwinding struct {
	id string

	candleInterval string

	previousClose float64
	previousOI    float64
	hasClose      bool

	hasResult bool
	lastValue bool

	lastTimestamp time.Time

	result LongUnwindingResult
}

func NewLongUnwinding(id string, cfg map[string]any) (*LongUnwinding, error) {
	if id == "" {
		return nil, fmt.Errorf("Long Unwinding name cannot be empty")
	}

	candleInterval, _ := cfg["candleInterval"].(string)

	return &LongUnwinding{
		id:             id,
		candleInterval: candleInterval,
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
	if candle == nil {
		return
	}

	closePrice := candle.GetClose()
	openInterest := candle.GetOpenInterest()
	timestamp := candle.GetTime()

	// Reject invalid candle data.
	if timestamp.IsZero() ||
		math.IsNaN(closePrice) ||
		math.IsInf(closePrice, 0) ||
		closePrice <= 0 ||
		math.IsNaN(openInterest) ||
		math.IsInf(openInterest, 0) ||
		openInterest < 0 ||
		(!r.lastTimestamp.IsZero() &&
			!timestamp.After(r.lastTimestamp)) {

		r.result = LongUnwindingResult{
			IsReady:   false,
			Timestamp: timestamp,
		}
		return
	}

	r.lastTimestamp = timestamp

	// First valid candle provides reference values.
	if !r.hasClose {
		r.previousClose = closePrice
		r.previousOI = openInterest
		r.hasClose = true

		r.result = LongUnwindingResult{
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
