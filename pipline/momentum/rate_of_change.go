package momentum

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: Rate of Change (ROC)
//
// pipeConfigs: [
//
//	{
//	  type: 'rateOfChange',
//	  name: 'roc',
//	  config: {
//	    period: 14,
//	    candleInterval: '1 minutes',
//	  }
//	}
//
// ],
//
// Available result:
//
//   - Rate of Change (ROC)
//
// Entry Condition:
//
//   ROC Crosses Above Zero
//   Previous <= 0 && Current > 0
//
// Exit Condition:
//
//   ROC Crosses Below Zero
//   Previous >= 0 && Current < 0
//
// Original Formula:
//
// PreviousClose = Close price from period candles ago
//
// ROC = ((CurrentClose - PreviousClose) / PreviousClose) * 100
//
// Equivalent Formula:
//
// ROC = ((CurrentClose / PreviousClose) - 1) * 100
//
// ROC > 0 = Positive price momentum
// ROC < 0 = Negative price momentum
// ROC = 0 = No percentage change
//
// Current represents current ROC.
// Previous represents previous ROC.
//
// With period 14, the first ROC is ready
// after 15 valid closing prices.
//
// The period is read from config.
// Zero is the reference level, not a configurable period.

type ROC struct {
	id string

	period         int
	candleInterval string

	closes []float64

	hasResult bool
	lastValue float64

	lastTimestamp time.Time

	result ROCResult
}

func NewROC(id string, cfg map[string]any) (*ROC, error) {
	if id == "" {
		return nil, fmt.Errorf("ROC name cannot be empty")
	}

	period := cfg["period"].(int)
	if period <= 0 {
		return nil, fmt.Errorf("ROC period must be positive: %d", period)
	}

	return &ROC{
		id:             id,
		period:         period,
		candleInterval: cfg["candleInterval"].(string),
	}, nil
}

func (r *ROC) Update(candle core.MarketCandle) any {
	r.ingest(candle)
	return r.result
}

func (r *ROC) Result() any {
	return r.result
}

func (r *ROC) ingest(candle core.MarketCandle) {
	if candle == nil {
		return
	}

	timestamp := candle.GetTime()
	closePrice := candle.GetClose()

	// Reject invalid, duplicate or out-of-order candles.
	if timestamp.IsZero() ||
		math.IsNaN(closePrice) ||
		math.IsInf(closePrice, 0) ||
		closePrice <= 0 ||
		(!r.lastTimestamp.IsZero() &&
			!timestamp.After(r.lastTimestamp)) {

		r.setNotReady(timestamp)
		return
	}

	r.lastTimestamp = timestamp

	// Store the latest closing price.
	r.closes = append(r.closes, closePrice)

	// Keep period + 1 closing prices.
	if len(r.closes) > r.period+1 {
		r.closes = r.closes[1:]
	}

	// ROC requires the current close and the
	// close from period candles ago.
	if len(r.closes) < r.period+1 {
		r.setNotReady(timestamp)
		return
	}

	previousClose := r.closes[0]

	// Protect against division by zero.
	if previousClose <= 0 {
		r.setNotReady(timestamp)
		return
	}

	// Calculate percentage Rate of Change.
	currentROC := ((closePrice / previousClose) - 1) * 100

	if math.IsNaN(currentROC) || math.IsInf(currentROC, 0) {
		r.setNotReady(timestamp)
		return
	}

	previousROC := currentROC

	if r.hasResult {
		previousROC = r.lastValue
	}

	r.result = ROCResult{
		IsReady:   true,
		Current:   currentROC,
		Previous:  previousROC,
		Timestamp: timestamp,
	}

	r.lastValue = currentROC
	r.hasResult = true
}

func (r *ROC) setNotReady(timestamp time.Time) {
	r.result = ROCResult{
		IsReady:   false,
		Timestamp: timestamp,
	}
}
