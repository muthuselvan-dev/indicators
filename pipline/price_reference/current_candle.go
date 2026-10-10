package price_reference

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: CurrentCandle
//
// Exposes the latest accepted candle as a typed
// OHLCV/OpenInterest snapshot.
//
// pipeConfigs: [
//
//	{
//	  type: 'currentCandle',
//	  name: 'currentCandle',
//	  config: {
//	    candleInterval: '1 minutes',
//	  }
//	}
//
// ],
//
// Available result:
//
//   - Current.Open
//   - Current.High
//   - Current.Low
//   - Current.Close
//   - Current.Volume
//   - Current.OpenInterest
//   - Current.Timestamp
//   - Previous (previous accepted candle snapshot)
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
//	Selected candle = current accepted candle.
//
//	No smoothing or mathematical formula.
//
// Warmup:
//
//	First valid candle is immediately ready.
//
//	First ready result:
//	  Previous = Current.
//
// Invalid candles:
//
//	Nil, invalid OHLCV/OI, duplicate timestamps,
//	and out-of-order timestamps are ignored.
//
// Note:
//
//	"Current" represents the candle supplied to Update.
//	The caller must ensure that candle is completed
//	if completed-candle semantics are required.
//
//	QuantMan exact reference semantics and parity
//	have not been verified.

type CurrentCandleConfig struct {
	CandleInterval string
}

// CurrentCandleSnapshot contains the accepted
// candle's OHLCV/OpenInterest values.
type CurrentCandleSnapshot struct {
	Open         float64
	High         float64
	Low          float64
	Close        float64
	Volume       float64
	OpenInterest float64
	Timestamp    time.Time
}

// CurrentCandleResult contains current and
// previous accepted candle snapshots.
type CurrentCandleResult struct {
	IsReady   bool
	Current   CurrentCandleSnapshot
	Previous  CurrentCandleSnapshot
	Timestamp time.Time
}

type CurrentCandle struct {
	id             string
	candleInterval string

	lastTimestamp time.Time

	hasResult bool
	result    CurrentCandleResult
}

func NewCurrentCandle(
	id string,
	cfg CurrentCandleConfig,
) (*CurrentCandle, error) {
	if id == "" {
		return nil, fmt.Errorf("CurrentCandle name cannot be empty")
	}

	return &CurrentCandle{
		id:             id,
		candleInterval: cfg.CandleInterval,
	}, nil
}

func (r *CurrentCandle) Update(
	candle core.MarketCandle,
) any {
	r.ingest(candle)
	return r.result
}

func (r *CurrentCandle) Result() any {
	return r.result
}

func (r *CurrentCandle) ingest(
	candle core.MarketCandle,
) {
	if !validCurrentCandle(
		candle,
		r.lastTimestamp,
	) {
		return
	}

	timestamp := candle.GetTime()

	selected := CurrentCandleSnapshot{
		Open:         candle.GetOpen(),
		High:         candle.GetHigh(),
		Low:          candle.GetLow(),
		Close:        candle.GetClose(),
		Volume:       candle.GetVolume(),
		OpenInterest: candle.GetOpenInterest(),
		Timestamp:    timestamp,
	}

	// First ready result uses Current as Previous.
	previous := selected

	if r.hasResult {
		previous = r.result.Current
	}

	r.result = CurrentCandleResult{
		IsReady:   true,
		Current:   selected,
		Previous:  previous,
		Timestamp: timestamp,
	}

	r.lastTimestamp = timestamp
	r.hasResult = true
}

// Own validation helper for CurrentCandle.
//
// No dependency on other indicator files.
func validCurrentCandle(
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

	// Reject duplicate and out-of-order timestamps.
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

	// Validate price values.
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

	// Validate volume and open interest.
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

	// Validate OHLC relationships.
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

var _ core.Indicator = (*CurrentCandle)(nil)
