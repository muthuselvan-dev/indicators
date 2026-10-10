package price_reference

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: PreviousNthCandle
//
// Exposes the Nth preceding completed candle
// as a typed OHLCV/OI snapshot.
//
// pipeConfigs: [
//
//	{
//	  type: 'previousNthCandle',
//	  name: 'previousNthCandle',
//	  config: {
//	    noOfCandles: 2,
//	    candleInterval: '1 minutes',
//	  }
//	}
//
// ],
//
// Available result:
//
//   - Current: Nth preceding candle snapshot
//   - Previous: Previously selected ready snapshot
//   - IsReady
//   - Timestamp
//
// Entry Condition:
//
//	Current.Close Is Above Current.Open:
//	  Current.Close > Current.Open
//
//	Current.Close Crosses Above Reference:
//	  Compare two consecutive ready snapshots.
//
// Exit Condition:
//
//	Current.Close Is Below Current.Open:
//	  Current.Close < Current.Open
//
// Original Reference Rule:
//
//	Selected candle = candle N positions before current.
//
//	No mathematical smoothing or hidden formula.
//
// Warmup:
//
//	Requires N + 1 valid candles.
//
//	Example for N = 2:
//
//	  Update Candle 1: Not ready
//	  Update Candle 2: Not ready
//	  Update Candle 3: Current = Candle 1
//	  Update Candle 4: Current = Candle 2
//
//	Previous equals Current on the first ready result.
//
// Note:
//
//	QuantMan exact anchor/reference semantics
//	and parity have not been verified.

type PreviousNthCandleConfig struct {
	NoOfCandles    int
	CandleInterval string
}

// PreviousNthCandleSnapshot stores OHLCV/OI values
// of a selected completed candle.
type PreviousNthCandleSnapshot struct {
	Open         float64
	High         float64
	Low          float64
	Close        float64
	Volume       float64
	OpenInterest float64
	Timestamp    time.Time
}

// PreviousNthCandleResult stores consecutive
// selected reference snapshots.
type PreviousNthCandleResult struct {
	IsReady   bool
	Current   PreviousNthCandleSnapshot
	Previous  PreviousNthCandleSnapshot
	Timestamp time.Time
}

type PreviousNthCandle struct {
	id             string
	index          int
	candleInterval string

	history []PreviousNthCandleSnapshot

	lastTimestamp time.Time

	hasResult bool
	result    PreviousNthCandleResult
}

func NewPreviousNthCandle(id string, cfg PreviousNthCandleConfig) (*PreviousNthCandle, error) {
	if id == "" {
		return nil, fmt.Errorf("PreviousNthCandle name cannot be empty")
	}

	if cfg.NoOfCandles <= 0 {
		return nil, fmt.Errorf(
			"PreviousNthCandle NoOfCandles must be positive: %d",
			cfg.NoOfCandles,
		)
	}

	return &PreviousNthCandle{
		id:             id,
		index:          cfg.NoOfCandles,
		candleInterval: cfg.CandleInterval,
	}, nil
}

func (r *PreviousNthCandle) Update(candle core.MarketCandle) any {
	r.ingest(candle)
	return r.result
}

func (r *PreviousNthCandle) Result() any {
	return r.result
}

func (r *PreviousNthCandle) ingest(candle core.MarketCandle) {

	if !validPreviousNthCandle(candle, r.lastTimestamp) {
		return
	}

	timestamp := candle.GetTime()

	currentSnapshot := PreviousNthCandleSnapshot{
		Open:         candle.GetOpen(),
		High:         candle.GetHigh(),
		Low:          candle.GetLow(),
		Close:        candle.GetClose(),
		Volume:       candle.GetVolume(),
		OpenInterest: candle.GetOpenInterest(),
		Timestamp:    timestamp,
	}

	r.lastTimestamp = timestamp

	// Add the accepted candle to history.
	r.history = append(r.history, currentSnapshot)

	// Keep at most N+1 candles.
	if len(r.history) > r.index+1 {
		r.history = r.history[1:]
	}

	// N+1 candles are required to select Nth previous.
	if len(r.history) <= r.index {
		r.result = PreviousNthCandleResult{
			IsReady:   false,
			Timestamp: timestamp,
		}
		return
	}

	// Select Nth preceding completed candle.
	selectedIndex := len(r.history) - 1 - r.index
	selected := r.history[selectedIndex]

	// First ready result uses Current as Previous.
	previous := selected

	if r.hasResult {
		previous = r.result.Current
	}

	r.result = PreviousNthCandleResult{
		IsReady:   true,
		Current:   selected,
		Previous:  previous,
		Timestamp: timestamp,
	}

	r.hasResult = true
}

// Own validation helper for PreviousNthCandle.
// No dependency on helpers from other indicator files.
func validPreviousNthCandle(candle core.MarketCandle, lastTimestamp time.Time) bool {

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
	volume := candle.GetVolume()
	openInterest := candle.GetOpenInterest()

	// Validate OHLC prices.
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

	// Validate Volume and Open Interest.
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

var _ core.Indicator = (*PreviousNthCandle)(nil)
