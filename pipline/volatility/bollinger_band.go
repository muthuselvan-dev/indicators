package volatility

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: Bollinger Band - rolling mean and standard-deviation envelope
//
// pipeConfigs: [
//
//	{
//	  type: 'bollingerBand',
//	  name: 'bollingerBand',
//	  config: {
//	    noOfCandles: 15,
//	    multiplier: 2,
//	    candleInterval: '3 minutes',
//	  }
//	}
//
// ],
//
// Available result:
//
//   - Middle (SMA)
//   - Upper
//   - Lower
//   - StandardDeviation
//   - Current (Middle)
//   - Previous
//   - IsReady
//   - Timestamp
//
// Entry Condition:
//
//	Close Is Above Upper Band:
//	  Close > Upper
//
//	Close Crosses Above Upper Band:
//	  PreviousClose <= PreviousUpper && Close > Upper
//
// Exit Condition:
//
//	Close Is Below Lower Band:
//	  Close < Lower
//
//	Close Crosses Below Lower Band:
//	  PreviousClose >= PreviousLower && Close < Lower
//
// Original Formula:
//
//	Middle = Sum(Close, N) / N
//
//	PopulationVariance = Sum((Close - Middle)^2, N) / N
//
//	StandardDeviation = Sqrt(PopulationVariance)
//
//	Upper = Middle + Multiplier * StandardDeviation
//
//	Lower = Middle - Multiplier * StandardDeviation
//
// BaseResult:
//
//	Current:
//	  Latest Middle Band (SMA).
//
//	Previous:
//	  Previous ready Middle Band.
//
//	IsReady:
//	  True after N valid candles.
//
//	Timestamp:
//	  Latest processed valid candle timestamp.
//
// Warmup:
//
//	First ready result requires N valid closing prices.
//
// Reference:
// https://chartschool.stockcharts.com/table-of-contents/technical-indicators-and-overlays/technical-overlays/bollinger-bands
//
// Note:
// Population standard deviation is used.
// QuantMan exact deviation convention and parity are not verified.

type BollingerBandConfig struct {
	NoOfCandles    int
	Multiplier     float64
	CandleInterval string
}

type BollingerBand struct {
	id             string
	period         int
	multiplier     float64
	candleInterval string

	closes []float64

	lastTimestamp time.Time

	hasResult bool
	lastValue float64

	result BollingerBandResult
}

func NewBollingerBand(id string, cfg BollingerBandConfig) (*BollingerBand, error) {
	if id == "" {
		return nil, fmt.Errorf("Bollinger Band name cannot be empty")
	}

	if cfg.NoOfCandles <= 0 {
		return nil, fmt.Errorf("Bollinger Band period must be positive")
	}

	if math.IsNaN(cfg.Multiplier) ||
		math.IsInf(cfg.Multiplier, 0) ||
		cfg.Multiplier <= 0 {
		return nil, fmt.Errorf("Bollinger Band multiplier must be positive and finite")
	}

	return &BollingerBand{
		id:             id,
		period:         cfg.NoOfCandles,
		multiplier:     cfg.Multiplier,
		candleInterval: cfg.CandleInterval,
	}, nil
}

func (b *BollingerBand) Update(candle core.MarketCandle) any {
	b.ingest(candle)
	return b.result
}

func (b *BollingerBand) Result() any {
	return b.result
}

func (b *BollingerBand) ingest(candle core.MarketCandle) {
	if !validBollingerBandCandle(candle, b.lastTimestamp) {
		return
	}

	timestamp := candle.GetTime()
	closePrice := candle.GetClose()

	b.lastTimestamp = timestamp

	// Maintain rolling closing prices.
	b.closes = append(b.closes, closePrice)

	if len(b.closes) > b.period {
		b.closes = b.closes[1:]
	}

	// Wait until N valid candles.
	if len(b.closes) < b.period {
		b.result = BollingerBandResult{
			BaseResult: BaseResult{
				IsReady:   false,
				Timestamp: timestamp,
			},
		}
		return
	}

	// Calculate Simple Moving Average.
	var sum float64

	for _, value := range b.closes {
		sum += value
	}

	mean := sum / float64(b.period)

	// Calculate population variance.
	var squaredSum float64

	for _, value := range b.closes {
		difference := value - mean
		squaredSum += difference * difference
	}

	variance := squaredSum / float64(b.period)

	// Calculate standard deviation.
	standardDeviation := math.Sqrt(variance)

	// Calculate upper and lower bands.
	upper := mean + b.multiplier*standardDeviation
	lower := mean - b.multiplier*standardDeviation

	// Reject non-finite calculation results.
	if math.IsNaN(mean) ||
		math.IsInf(mean, 0) ||
		math.IsNaN(standardDeviation) ||
		math.IsInf(standardDeviation, 0) ||
		math.IsNaN(upper) ||
		math.IsInf(upper, 0) ||
		math.IsNaN(lower) ||
		math.IsInf(lower, 0) {
		return
	}

	// Previous equals Current on the first ready candle.
	previous := mean

	if b.hasResult {
		previous = b.lastValue
	}

	b.result = BollingerBandResult{
		BaseResult: BaseResult{
			IsReady:   true,
			Current:   mean,
			Previous:  previous,
			Timestamp: timestamp,
		},
		Middle:            mean,
		Upper:             upper,
		Lower:             lower,
		StandardDeviation: standardDeviation,
	}

	b.lastValue = mean
	b.hasResult = true
}

// Own validation helper for Bollinger Band.
// Does not depend on helpers from other indicator files.
func validBollingerBandCandle(
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

	// Reject duplicate or older candles.
	if !lastTimestamp.IsZero() &&
		!timestamp.After(lastTimestamp) {
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
