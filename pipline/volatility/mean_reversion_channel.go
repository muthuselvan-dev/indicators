package volatility

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: MRC (Mean Reversion Channel)
//
// This implementation uses a rolling Simple Moving Average
// with population-standard-deviation channel bands.
//
// pipeConfigs: [
//
//	{
//	  type: 'meanReversionChannel',
//	  name: 'meanReversionChannel',
//	  config: {
//	    period: 14,
//	    multiplier: 2,
//	    candleInterval: '1 minutes'
//	  }
//	}
//
// ]
//
// Available result:
//
//   - Current (Middle)
//   - Previous (Previous ready Middle)
//   - Middle
//   - Upper
//   - Lower
//   - StandardDeviation
//   - IsReady
//   - Timestamp
//
// Entry Condition:
//
//   Close Is Below Lower:
//     Close < Lower
//
//   Close Crosses Below Lower:
//     PreviousClose >= PreviousLower && Close < Lower
//
// Exit Condition:
//
//   Close Is Above Upper:
//     Close > Upper
//
//   Close Crosses Above Upper:
//     PreviousClose <= PreviousUpper && Close > Upper
//
// Original Formula / Explicit Interpretation:
//
//   Middle = Sum(Close, Period) / Period
//
//   Variance = Sum((Close - Middle)^2) / Period
//
//   StandardDeviation = Sqrt(Variance)
//
//   Upper = Middle + Multiplier * StandardDeviation
//
//   Lower = Middle - Multiplier * StandardDeviation
//
// BaseResult:
//
//   Current:
//     Latest Middle value.
//
//   Previous:
//     Previous ready Middle value.
//
// Warmup:
//
//   First ready result requires Period valid candles.
//
// Important:
//
//   MaxBodyRatio is retained for config compatibility.
//   It is not used by this SMA/standard-deviation formula.
//
//   Entry and Exit conditions are examples, not
//   guaranteed trading signals.
//
//   QuantMan-specific MRC semantics and calculation
//   parity have not been verified.

type MeanReversionChannelConfig struct {
	Period         int
	Multiplier     float64
	MaxBodyRatio   float64
	CandleInterval string
}

type MeanReversionChannelResult struct {
	BaseResult
	Middle            float64
	Upper             float64
	Lower             float64
	StandardDeviation float64
}

type MeanReversionChannel struct {
	id             string
	period         int
	multiplier     float64
	candleInterval string

	closes []float64

	lastTimestamp time.Time

	hasResult bool
	lastValue float64

	result MeanReversionChannelResult
}

func NewMeanReversionChannel(id string, cfg MeanReversionChannelConfig) (*MeanReversionChannel, error) {
	if id == "" {
		return nil, fmt.Errorf("MRC name cannot be empty")
	}

	if cfg.Period <= 0 {
		return nil, fmt.Errorf("MRC period must be positive")
	}

	if math.IsNaN(cfg.Multiplier) ||
		math.IsInf(cfg.Multiplier, 0) ||
		cfg.Multiplier < 0 {
		return nil, fmt.Errorf("MRC multiplier must be finite and non-negative")
	}

	if math.IsNaN(cfg.MaxBodyRatio) ||
		math.IsInf(cfg.MaxBodyRatio, 0) ||
		cfg.MaxBodyRatio < 0 {
		return nil, fmt.Errorf("MRC MaxBodyRatio must be finite and non-negative")
	}

	return &MeanReversionChannel{
		id:             id,
		period:         cfg.Period,
		multiplier:     cfg.Multiplier,
		candleInterval: cfg.CandleInterval,
	}, nil
}

func (r *MeanReversionChannel) Update(candle core.MarketCandle) any {
	r.ingest(candle)
	return r.result
}

func (r *MeanReversionChannel) Result() any {
	return r.result
}

func (r *MeanReversionChannel) ingest(candle core.MarketCandle) {
	
	if !validMeanReversionChannelCandle(candle, r.lastTimestamp) {
		return
	}

	timestamp := candle.GetTime()
	closePrice := candle.GetClose()

	r.lastTimestamp = timestamp

	// Maintain rolling closing prices.
	r.closes = append(r.closes, closePrice)

	if len(r.closes) > r.period {
		r.closes = r.closes[1:]
	}

	// Wait until the configured period is ready.
	if len(r.closes) < r.period {
		r.result = MeanReversionChannelResult{
			BaseResult: BaseResult{
				IsReady:   false,
				Timestamp: timestamp,
			},
		}
		return
	}

	// Calculate rolling SMA.
	var sum float64

	for _, value := range r.closes {
		sum += value
	}

	middle := sum / float64(r.period)

	// Calculate population variance.
	var squaredSum float64

	for _, value := range r.closes {
		difference := value - middle
		squaredSum += difference * difference
	}

	variance := squaredSum / float64(r.period)
	standardDeviation := math.Sqrt(variance)

	// Calculate channel bands.
	upper := middle + r.multiplier*standardDeviation
	lower := middle - r.multiplier*standardDeviation

	// Reject non-finite results.
	for _, value := range []float64{
		middle,
		standardDeviation,
		upper,
		lower,
	} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return
		}
	}

	// Previous equals Current on first ready candle.
	previous := middle

	if r.hasResult {
		previous = r.lastValue
	}

	r.result = MeanReversionChannelResult{
		BaseResult: BaseResult{
			IsReady:   true,
			Current:   middle,
			Previous:  previous,
			Timestamp: timestamp,
		},
		Middle:            middle,
		Upper:             upper,
		Lower:             lower,
		StandardDeviation: standardDeviation,
	}

	r.lastValue = middle
	r.hasResult = true
}

// Own validation helper for Mean Reversion Channel.
// No dependency on helpers from other indicator files.
func validMeanReversionChannelCandle(candle core.MarketCandle, lastTimestamp time.Time) bool {
	if candle == nil {
		return false
	}

	timestamp := candle.GetTime()

	if timestamp.IsZero() {
		return false
	}

	// Reject duplicate and out-of-order candles.
	if !lastTimestamp.IsZero() && !timestamp.After(lastTimestamp) {
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

var _ core.Indicator = (*MeanReversionChannel)(nil)
