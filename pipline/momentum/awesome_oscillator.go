package momentum

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: Awesome Oscillator (AO)
//
// Awesome Oscillator measures the difference
// between fast and slow simple moving averages
// of the median price.
//
// pipeConfigs: [
//
//	{
//	  type: 'awesomeOscillator',
//	  name: 'ao',
//	  config: {
//	    fastPeriod: 5,
//	    slowPeriod: 34,
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
//	AO Crosses Above Zero:
//	  Previous <= 0 && Current > 0
//
// Exit Condition:
//
//	AO Crosses Below Zero:
//	  Previous >= 0 && Current < 0
//
// Original Formula:
//
//	MedianPrice = (High + Low) / 2
//
//	FastSMA = SMA(MedianPrice, fastPeriod)
//
//	SlowSMA = SMA(MedianPrice, slowPeriod)
//
//	AO = FastSMA - SlowSMA
//
// Interpretation:
//
//	AO > 0: Fast momentum exceeds slow momentum.
//	AO < 0: Fast momentum is below slow momentum.
//	AO = 0: Both averages are equal.
//
// Warmup:
//
//	Requires slowPeriod valid candles.
//
//	For fastPeriod=5 and slowPeriod=34:
//	  First ready AO is on candle 34.
//
// Previous:
//
//	First ready value:
//	  Previous = Current.
//
//	Subsequent values:
//	  Previous = prior ready AO.
//
// Validation:
//
//	Invalid, duplicate and out-of-order candles
//	are ignored without modifying the last result.
//
// Session:
//
//	No daily reset. Rolling history continues
//	across trading sessions.
//
// Reference:
//
//	Bill Williams Awesome Oscillator.
//
// Note:
//
//	QuantMan calculation parity is not yet verified.

type AOConfig struct {
	FastPeriod     int
	SlowPeriod     int
	CandleInterval string
}

type AO struct {
	id string

	fastPeriod     int
	slowPeriod     int
	candleInterval string

	medianPrices []float64

	lastTimestamp time.Time

	lastValue float64
	hasResult bool

	result AOResult
}

func NewAO(id string, cfg map[string]any) (*AO, error) {

	if id == "" {
		return nil, fmt.Errorf(
			"Awesome Oscillator name cannot be empty",
		)
	}

	fastPeriod, ok := cfg["fastPeriod"].(int)

	if !ok || fastPeriod <= 0 {
		return nil, fmt.Errorf(
			"Awesome Oscillator fastPeriod must be positive",
		)
	}

	slowPeriod, ok := cfg["slowPeriod"].(int)

	if !ok || slowPeriod <= 0 {
		return nil, fmt.Errorf(
			"Awesome Oscillator slowPeriod must be positive",
		)
	}

	if fastPeriod >= slowPeriod {
		return nil, fmt.Errorf(
			"Awesome Oscillator fastPeriod must be smaller than slowPeriod",
		)
	}

	candidateInterval, ok := cfg["candleInterval"].(string)

	if !ok || candidateInterval == "" {
		return nil, fmt.Errorf(
			"Awesome Oscillator candleInterval must be a non-empty string",
		)
	}

	return &AO{
		id:             id,
		fastPeriod:     fastPeriod,
		slowPeriod:     slowPeriod,
		candleInterval: candidateInterval,
		medianPrices:   make([]float64, 0, slowPeriod),
	}, nil
}

func (a *AO) Update(candle core.MarketCandle) any {
	a.ingest(candle)
	return a.result
}

func (a *AO) Result() any {
	return a.result
}

func (a *AO) ingest(candle core.MarketCandle) {

	if !validAwesomeOscillatorCandle(
		candle,
		a.lastTimestamp,
	) {
		return
	}

	timestamp := candle.GetTime()

	high := candle.GetHigh()
	low := candle.GetLow()

	// Calculate median price.
	// Half-first arithmetic reduces overflow risk.
	medianPrice := high/2.0 + low/2.0

	if math.IsNaN(medianPrice) ||
		math.IsInf(medianPrice, 0) {
		return
	}

	// Build a candidate rolling window first.
	// Commit only after calculations are valid.
	window := make([]float64, 0, a.slowPeriod)

	if len(a.medianPrices) >= a.slowPeriod {
		window = append(
			window,
			a.medianPrices[1:]...,
		)
	} else {
		window = append(window, a.medianPrices...)
	}

	window = append(window, medianPrice)

	// Warmup phase.
	if len(window) < a.slowPeriod {
		a.medianPrices = window
		a.lastTimestamp = timestamp

		a.result = AOResult{
			IsReady:   false,
			Timestamp: timestamp,
		}
		return
	}

	// Calculate slow SMA.
	slowSum := 0.0

	for _, price := range window {
		slowSum += price
	}

	// Calculate fast SMA.
	fastSum := 0.0

	start := len(window) - a.fastPeriod

	for i := start; i < len(window); i++ {
		fastSum += window[i]
	}

	fastSMA := fastSum / float64(a.fastPeriod)
	slowSMA := slowSum / float64(a.slowPeriod)

	currentAO := fastSMA - slowSMA

	// Reject non-finite calculations.
	if math.IsNaN(fastSMA) ||
		math.IsInf(fastSMA, 0) ||
		math.IsNaN(slowSMA) ||
		math.IsInf(slowSMA, 0) ||
		math.IsNaN(currentAO) ||
		math.IsInf(currentAO, 0) {
		return
	}

	// First ready result uses Current as Previous.
	previousAO := currentAO

	if a.hasResult {
		previousAO = a.lastValue
	}

	// Commit accepted candle and result.
	a.medianPrices = window
	a.lastTimestamp = timestamp

	a.result = AOResult{
		IsReady:   true,
		Current:   currentAO,
		Previous:  previousAO,
		Timestamp: timestamp,
	}

	a.lastValue = currentAO
	a.hasResult = true
}

// Own validation helper for Awesome Oscillator.
//
// No dependency on other indicator files.

func validAwesomeOscillatorCandle(candle core.MarketCandle, lastTimestamp time.Time) bool {
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

var _ core.Indicator = (*AO)(nil)
