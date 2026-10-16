package momentum

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: Stochastic Oscillator
//
// pipeConfigs: [
//
//	{
//	  type: 'stochasticOscillator',
//	  name: 'stochastic',
//	  config: {
//	    period: 14,
//	    smoothK: 3,
//	    smoothD: 3,
//	    candleInterval: '1 minutes',
//	  }
//	}
//
// ],
//
// Available result:
//
//   - Stochastic %K
//   - Stochastic %D
//
// Entry Condition:
//
//   %K Crosses Above %D
//   PreviousK <= PreviousD && K > D
//
// Exit Condition:
//
//   %K Crosses Below %D
//   PreviousK >= PreviousD && K < D
//
// Original Formula:
//
// LowestLow = Lowest Low over period candles
// HighestHigh = Highest High over period candles
//
// RawK = 100 * (Close - LowestLow) /
//              (HighestHigh - LowestLow)
//
// SmoothedK = SMA(RawK, smoothK)
//
// D = SMA(SmoothedK, smoothD)
//
// When HighestHigh == LowestLow:
//   RawK = 50 (neutral fallback)
//
// Current represents smoothed %K.
// Previous represents previous smoothed %K.
//
// First complete K/D result is ready after
// period + smoothK + smoothD - 2 valid candles.

type StochasticOscillator struct {
	id string

	period         int
	smoothK        int
	smoothD        int
	candleInterval string

	highs []float64
	lows  []float64

	rawKValues []float64
	kValues    []float64

	lastTimestamp time.Time

	hasResult bool
	lastK     float64
	lastD     float64

	result StochasticOscillatorResult
}

func NewStochasticOscillator(id string, cfg map[string]any) (*StochasticOscillator, error) {
	if id == "" {
		return nil, fmt.Errorf("stochastic oscillator name cannot be empty")
	}

	period, ok := cfg["period"].(int)
	if !ok || period <= 0 {
		return nil, fmt.Errorf("stochastic oscillator period must be a positive integer")
	}

	smoothK, ok := cfg["smoothK"].(int)
	if !ok || smoothK <= 0 {
		return nil, fmt.Errorf("stochastic oscillator smoothK must be a positive integer")
	}

	smoothD, ok := cfg["smoothD"].(int)
	if !ok || smoothD <= 0 {
		return nil, fmt.Errorf("stochastic oscillator smoothD must be a positive integer")
	}

	// Validate candleInterval (non-empty string).
	candleInterval, ok := cfg["candleInterval"].(string)
	if !ok || candleInterval == "" {
		return nil, fmt.Errorf("stochastic oscillator periods must be positive")
	}

	return &StochasticOscillator{
		id:             id,
		period:         period,
		smoothK:        smoothK,
		smoothD:        smoothD,
		candleInterval: candleInterval,
	}, nil
}

func (s *StochasticOscillator) Update(candle core.MarketCandle) any {
	s.ingest(candle)
	return s.result
}

func (s *StochasticOscillator) Result() any {
	return s.result
}

func (s *StochasticOscillator) ingest(candle core.MarketCandle) {
	if candle == nil {
		return
	}

	timestamp := candle.GetTime()
	high := candle.GetHigh()
	low := candle.GetLow()
	closePrice := candle.GetClose()

	// Reject invalid or duplicate candles.
	if timestamp.IsZero() ||
		!validStochasticNumber(high) ||
		!validStochasticNumber(low) ||
		!validStochasticNumber(closePrice) ||
		low <= 0 ||
		high < low ||
		closePrice < low ||
		closePrice > high ||
		(!s.lastTimestamp.IsZero() &&
			!timestamp.After(s.lastTimestamp)) {

		s.setNotReady(timestamp)
		return
	}

	s.lastTimestamp = timestamp

	// Maintain rolling high and low windows.
	s.highs = append(s.highs, high)
	s.lows = append(s.lows, low)

	if len(s.highs) > s.period {
		s.highs = s.highs[1:]
		s.lows = s.lows[1:]
	}

	if len(s.highs) < s.period {
		s.setNotReady(timestamp)
		return
	}

	highestHigh := s.highs[0]
	lowestLow := s.lows[0]

	for i := 1; i < len(s.highs); i++ {
		if s.highs[i] > highestHigh {
			highestHigh = s.highs[i]
		}

		if s.lows[i] < lowestLow {
			lowestLow = s.lows[i]
		}
	}

	// Calculate raw %K.
	rawK := 50.0

	if highestHigh != lowestLow {
		rawK = 100 * (closePrice - lowestLow) / (highestHigh - lowestLow)
	}

	s.rawKValues = append(s.rawKValues, rawK)

	if len(s.rawKValues) > s.smoothK {
		s.rawKValues = s.rawKValues[1:]
	}

	if len(s.rawKValues) < s.smoothK {
		s.setNotReady(timestamp)
		return
	}

	// Smooth raw %K using SMA.
	currentK := stochasticAverage(s.rawKValues)

	s.kValues = append(s.kValues, currentK)

	if len(s.kValues) > s.smoothD {
		s.kValues = s.kValues[1:]
	}

	if len(s.kValues) < s.smoothD {
		s.setNotReady(timestamp)
		return
	}

	// Calculate %D using SMA of %K.
	currentD := stochasticAverage(s.kValues)

	if !validStochasticNumber(currentK) || !validStochasticNumber(currentD) {
		s.setNotReady(timestamp)
		return
	}

	previousK := currentK
	previousD := currentD

	if s.hasResult {
		previousK = s.lastK
		previousD = s.lastD
	}

	s.result = StochasticOscillatorResult{
		IsReady:   true,
		Current:   currentK,
		Previous:  previousK,
		K:         currentK,
		D:         currentD,
		PreviousK: previousK,
		PreviousD: previousD,
		Timestamp: timestamp,
	}

	s.lastK = currentK
	s.lastD = currentD
	s.hasResult = true
}

func (s *StochasticOscillator) setNotReady(timestamp time.Time) {
	s.result = StochasticOscillatorResult{
		IsReady:   false,
		Timestamp: timestamp,
	}
}

func stochasticAverage(values []float64) float64 {
	var sum float64

	for _, value := range values {
		sum += value
	}

	return sum / float64(len(values))
}

func validStochasticNumber(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
