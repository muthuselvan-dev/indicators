package volatility

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// Indicator Name: Bollinger Band
//
// Pipe Config:
//
//	{
//	  Type: "bollingerBand",
//	  Name: "bollingerBand",
//	  Config: map[string]any{
//	    "period":         15,
//	    "multiplier":     2.0,
//	    "candleInterval": "3 minutes",
//	  },
//	}
//
// Entry Condition (example):
//
//	Close Is Above Upper Band:
//	  Close > Upper
//
//	Close Crosses Above Upper Band:
//	  PreviousClose <= PreviousUpper && Close > Upper
//
// Exit Condition (example):
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
// Result:
//	Middle, Upper, Lower, StandardDeviation,
//	Current, Previous, IsReady, Timestamp
//
// Note:
//	Population standard deviation is used.
//	QuantMan exact parity is not yet verified.

type BollingerBandConfig struct {
	Period         int
	Multiplier     float64
	CandleInterval string
}

type BollingerBand struct {
	id  string
	cfg BollingerBandConfig

	closes []float64

	lastTimestamp time.Time

	hasResult bool
	lastValue float64

	result BollingerBandResult
}

func NewBollingerBand(
	id string,
	cfg map[string]any,
) (*BollingerBand, error) {

	if id == "" {
		return nil, fmt.Errorf(
			"bollingerBand: indicator id is required",
		)
	}

	config := BollingerBandConfig{
		Period:         15,
		Multiplier:     2.0,
		CandleInterval: "3 minutes",
	}

	if value, exists := cfg["period"]; exists {
		period, ok := value.(int)
		if !ok {
			return nil, fmt.Errorf(
				"bollingerBand: period must be an integer",
			)
		}
		config.Period = period
	}

	if value, exists := cfg["multiplier"]; exists {
		switch v := value.(type) {
		case int:
			config.Multiplier = float64(v)
		case float64:
			config.Multiplier = v
		default:
			return nil, fmt.Errorf(
				"bollingerBand: multiplier must be numeric",
			)
		}
	}

	if value, exists := cfg["candleInterval"]; exists {
		interval, ok := value.(string)
		if !ok || interval == "" {
			return nil, fmt.Errorf(
				"bollingerBand: invalid candleInterval",
			)
		}
		config.CandleInterval = interval
	}

	if config.Period <= 0 {
		return nil, fmt.Errorf(
			"bollingerBand: period must be positive",
		)
	}

	if math.IsNaN(config.Multiplier) ||
		math.IsInf(config.Multiplier, 0) ||
		config.Multiplier <= 0 {
		return nil, fmt.Errorf(
			"bollingerBand: multiplier must be positive and finite",
		)
	}

	return &BollingerBand{
		id:  id,
		cfg: config,
	}, nil
}

func (b *BollingerBand) Update(
	candle core.MarketCandle,
) any {
	b.ingest(candle)
	return b.result
}

func (b *BollingerBand) Result() any {
	return b.result
}

func (b *BollingerBand) ingest(
	candle core.MarketCandle,
) {
	if !validBollingerBandCandle(
		candle,
		b.lastTimestamp,
	) {
		b.result.IsReady = false
		return
	}

	timestamp := candle.GetTime()
	closePrice := candle.GetClose()

	b.lastTimestamp = timestamp

	// Maintain rolling closing prices.
	b.closes = append(b.closes, closePrice)

	if len(b.closes) > b.cfg.Period {
		b.closes = b.closes[1:]
	}

	// Wait until N valid candles.
	if len(b.closes) < b.cfg.Period {
		b.result = BollingerBandResult{
			BaseResult: BaseResult{
				IsReady:   false,
				Timestamp: timestamp,
			},
		}
		return
	}

	// Simple Moving Average.
	var sum float64

	for _, value := range b.closes {
		sum += value
	}

	mean := sum / float64(b.cfg.Period)

	// Population variance.
	var squaredSum float64

	for _, value := range b.closes {
		difference := value - mean
		squaredSum += difference * difference
	}

	variance := squaredSum / float64(b.cfg.Period)

	standardDeviation := math.Sqrt(variance)

	upper := mean + b.cfg.Multiplier*standardDeviation
	lower := mean - b.cfg.Multiplier*standardDeviation

	if math.IsNaN(mean) ||
		math.IsInf(mean, 0) ||
		math.IsNaN(standardDeviation) ||
		math.IsInf(standardDeviation, 0) ||
		math.IsNaN(upper) ||
		math.IsInf(upper, 0) ||
		math.IsNaN(lower) ||
		math.IsInf(lower, 0) {
		b.result.IsReady = false
		return
	}

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

// Independent validation helper.
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
