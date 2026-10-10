package price_reference

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: Previous Value Indecision
//
// Returns the body-to-range ratio of the
// immediately preceding valid candle.
//
// pipeConfigs: [
//
//	{
//	  type: 'previousValueIndecision',
//	  name: 'previousValueIndecision',
//	  config: {
//	    period: 14,
//	    multiplier: 0,
//	    maxBodyRatio: 0,
//	    candleInterval: '1 minutes',
//	  },
//	}
//
// ],
//
// Available result:
//
//   - Current
//   - Previous
//   - IsReady
//   - Timestamp
//
// Entry Condition:
//
//	Is Above threshold:
//	  Current > threshold
//
//	Crosses Above threshold:
//	  Previous <= threshold && Current > threshold
//
// Exit Condition:
//
//	Is Below threshold:
//	  Current < threshold
//
//	Crosses Below threshold:
//	  Previous >= threshold && Current < threshold
//
// Original Formula / Explicit Interpretation:
//
//	PreviousValueIndecision =
//	  abs(PreviousClose - PreviousOpen) /
//	  (PreviousHigh - PreviousLow)
//
// Warmup:
//
//	Requires one preceding valid candle with
//	a nonzero High-Low range.
//
//	No period-based smoothing is applied.
//
// Previous Result:
//
//	First ready result:
//	  Previous = Current
//
//	Later ready results:
//	  Previous = prior ready Current
//
// Configuration Compatibility:
//
//	Period, Multiplier and MaxBodyRatio are retained
//	to preserve the supplied configuration API.
//
//	They do not change the stated ratio formula.
//
// Validation:
//
//	Invalid, duplicate and out-of-order candles
//	are ignored.
//
//	A zero-range previous candle produces
//	a non-ready result because division by zero
//	is undefined.
//
// Reference:
//
//	Standard OHLC body-to-range arithmetic.
//
//	QuantMan-specific semantics and parity
//	have not been verified.

type PreviousValueIndecisionConfig struct {
	Period         int
	Multiplier     float64
	MaxBodyRatio   float64
	CandleInterval string
}

type previousValueIndecisionCandle struct {
	open  float64
	high  float64
	low   float64
	close float64
}

type PreviousValueIndecision struct {
	id  string
	cfg PreviousValueIndecisionConfig

	lastTime time.Time

	previousCandle    previousValueIndecisionCandle
	hasPreviousCandle bool

	hasResult bool
	result    ScalarResult
}

func NewPreviousValueIndecision(id string, cfg PreviousValueIndecisionConfig) (*PreviousValueIndecision, error) {
	if id == "" {
		return nil, fmt.Errorf(
			"Previous Value Indecision: name cannot be empty",
		)
	}

	if cfg.Period <= 0 {
		return nil, fmt.Errorf(
			"Previous Value Indecision: period must be positive",
		)
	}

	if math.IsNaN(cfg.Multiplier) ||
		math.IsInf(cfg.Multiplier, 0) ||
		cfg.Multiplier < 0 {
		return nil, fmt.Errorf(
			"Previous Value Indecision: invalid multiplier",
		)
	}

	if math.IsNaN(cfg.MaxBodyRatio) ||
		math.IsInf(cfg.MaxBodyRatio, 0) ||
		cfg.MaxBodyRatio < 0 {
		return nil, fmt.Errorf(
			"Previous Value Indecision: invalid max body ratio",
		)
	}

	return &PreviousValueIndecision{
		id:  id,
		cfg: cfg,
	}, nil
}

func (r *PreviousValueIndecision) Update(candle core.MarketCandle) any {
	r.ingest(candle)
	return r.result
}

func (r *PreviousValueIndecision) Result() any {
	return r.result
}

func (r *PreviousValueIndecision) ingest(candle core.MarketCandle) {
	if !validPreviousValueIndecisionCandle(
		candle,
		r.lastTime,
	) {
		return
	}

	timestamp := candle.GetTime()

	currentCandle := previousValueIndecisionCandle{
		open:  candle.GetOpen(),
		high:  candle.GetHigh(),
		low:   candle.GetLow(),
		close: candle.GetClose(),
	}

	r.lastTime = timestamp

	// First valid candle provides the initial
	// reference for the next update.
	if !r.hasPreviousCandle {
		r.previousCandle = currentCandle
		r.hasPreviousCandle = true

		r.result = ScalarResult{
			IsReady:   false,
			Timestamp: timestamp,
		}
		return
	}

	// Always calculate from the preceding candle.
	previousCandle := r.previousCandle

	// Advance reference to the newly accepted candle.
	r.previousCandle = currentCandle

	candleRange := previousCandle.high - previousCandle.low

	// Ratio is undefined for a zero-range candle.
	if candleRange <= 0 {
		r.result = ScalarResult{
			IsReady:   false,
			Timestamp: timestamp,
		}
		return
	}

	bodySize := math.Abs(
		previousCandle.close - previousCandle.open,
	)

	current := bodySize / candleRange

	if math.IsNaN(current) ||
		math.IsInf(current, 0) {
		r.result = ScalarResult{
			IsReady:   false,
			Timestamp: timestamp,
		}
		return
	}

	// First ready result uses Current as Previous.
	previous := current

	if r.hasResult {
		previous = r.result.Current
	}

	r.result = ScalarResult{
		IsReady:   true,
		Current:   current,
		Previous:  previous,
		Timestamp: timestamp,
	}

	r.hasResult = true
}

// Own validation helper.
//
// No dependency on helpers from other indicator files.
func validPreviousValueIndecisionCandle(candle core.MarketCandle, lastTime time.Time) bool {

	if candle == nil {
		return false
	}

	timestamp := candle.GetTime()

	if timestamp.IsZero() {
		return false
	}

	// Reject duplicate and out-of-order timestamps.
	if !lastTime.IsZero() &&
		!timestamp.After(lastTime) {
		return false
	}

	open := candle.GetOpen()
	high := candle.GetHigh()
	low := candle.GetLow()
	closePrice := candle.GetClose()

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

var _ core.Indicator = (*PreviousValueIndecision)(nil)
