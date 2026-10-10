package price_reference

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: PreviousCandle
//
// Exposes the immediately preceding completed candle
// as a typed OHLCV/OI snapshot.
//
// pipeConfigs: [
//
//	{
//	  type: 'previousCandle',
//	  name: 'previousCandle',
//	  config: {
//	    candleInterval: '1 minutes',
//	  }
//	}
//
// ],
//
// Available result:
//
//   - Current: Previous completed candle snapshot
//   - Previous: Snapshot returned by preceding ready update
//   - IsReady
//   - Timestamp
//
// Entry Condition:
//
//	Current.Close Is Above Current.Open:
//	  Current.Close > Current.Open
//
//	Current.Close Crosses Above Reference:
//	  Compare consecutive ready snapshots.
//
// Exit Condition:
//
//	Current.Close Is Below Current.Open:
//	  Current.Close < Current.Open
//
// Original Reference Rule:
//
//	Selected candle = one valid candle before current.
//
//	No smoothing or mathematical indicator formula.
//
// Warmup:
//
//	Requires two valid candles.
//
//	First ready result:
//	  Previous = Current.
//
// Important:
//
//	Invalid, duplicate and out-of-order candles
//	do not advance the reference.
//
//	QuantMan exact reference semantics are not verified.

type PreviousCandleConfig struct {
	CandleInterval string
}

// PreviousCandleSnapshot stores one completed candle.
type PreviousCandleSnapshot struct {
	Open         float64
	High         float64
	Low          float64
	Close        float64
	Volume       float64
	OpenInterest float64
	Timestamp    time.Time
}

type PreviousCandle struct {
	id             string
	candleInterval string

	lastTimestamp time.Time

	lastCandle PreviousCandleSnapshot
	hasCandle  bool

	result    PreviousCandleResult
	hasResult bool
}

func NewPreviousCandle(id string, cfg PreviousCandleConfig) (*PreviousCandle, error) {

	if id == "" {
		return nil, fmt.Errorf("PreviousCandle name cannot be empty")
	}

	return &PreviousCandle{
		id:             id,
		candleInterval: cfg.CandleInterval,
	}, nil
}

func (r *PreviousCandle) Update(candle core.MarketCandle) any {
	r.ingest(candle)
	return r.result
}

func (r *PreviousCandle) Result() any {
	return r.result
}

func (r *PreviousCandle) ingest(candle core.MarketCandle) {

	if !validPreviousCandle(candle, r.lastTimestamp) {
		return
	}

	timestamp := candle.GetTime()

	currentSnapshot := PreviousCandleSnapshot{
		Open:         candle.GetOpen(),
		High:         candle.GetHigh(),
		Low:          candle.GetLow(),
		Close:        candle.GetClose(),
		Volume:       candle.GetVolume(),
		OpenInterest: candle.GetOpenInterest(),
		Timestamp:    timestamp,
	}

	r.lastTimestamp = timestamp

	// First candle is only stored for reference.
	if !r.hasCandle {
		r.lastCandle = currentSnapshot
		r.hasCandle = true

		r.result = PreviousCandleResult{
			IsReady:   false,
			Timestamp: timestamp,
		}
		return
	}

	// Select the preceding completed candle.
	selected := r.lastCandle

	// First ready result uses selected as Previous.
	previous := selected

	if r.hasResult {
		previous = r.result.Current
	}

	r.result = PreviousCandleResult{
		IsReady:   true,
		Current:   selected,
		Previous:  previous,
		Timestamp: timestamp,
	}

	r.hasResult = true

	// Save latest candle for the next update.
	r.lastCandle = currentSnapshot
}

// Own validation helper for PreviousCandle.
// No dependency on other indicator files.
func validPreviousCandle(candle core.MarketCandle, lastTimestamp time.Time) bool {
	if candle == nil {
		return false
	}

	timestamp := candle.GetTime()

	if timestamp.IsZero() {
		return false
	}

	// Reject duplicate or out-of-order candles.
	if !lastTimestamp.IsZero() &&
		!timestamp.After(lastTimestamp) {
		return false
	}

	open := candle.GetOpen()
	high := candle.GetHigh()
	low := candle.GetLow()
	closePrice := candle.GetClose()
	volume := candle.GetVolume()
	openInterest := candle.GetOpenInterest()

	for _, value := range []float64{
		open,
		high,
		low,
		closePrice,
	} {
		if math.IsNaN(value) ||
			math.IsInf(value, 0) ||
			value <= 0 {
			return false
		}
	}

	for _, value := range []float64{
		volume,
		openInterest,
	} {
		if math.IsNaN(value) ||
			math.IsInf(value, 0) ||
			value < 0 {
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

var _ core.Indicator = (*PreviousCandle)(nil)
