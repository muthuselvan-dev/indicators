package momentum

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: Stochastic Relative Strength Index (Stochastic RSI)
//
// pipeConfigs: [
//
//	{
//	  type: 'stochasticRelativeStrengthIndex',
//	  name: 'stochRsi',
//	  config: {
//	    rsiPeriod: 14,
//	    stochasticPeriod: 14,
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
//   - Stochastic RSI %K
//   - Stochastic RSI %D
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
// Original Formula (Wilder RSI + Stochastic RSI):
//
// Change = CurrentClose - PreviousClose
//
// Gain = Max(Change, 0)
// Loss = Max(-Change, 0)
//
// Initial Average Gain = Sum(Gains, rsiPeriod) / rsiPeriod
// Initial Average Loss = Sum(Losses, rsiPeriod) / rsiPeriod
//
// Subsequent Average Gain =
//   ((PreviousAverageGain * (rsiPeriod - 1)) + Gain) / rsiPeriod
//
// Subsequent Average Loss =
//   ((PreviousAverageLoss * (rsiPeriod - 1)) + Loss) / rsiPeriod
//
// RSI = 100 - (100 / (1 + AverageGain / AverageLoss))
//
// When AverageLoss = 0:
//   RSI = 100 if AverageGain > 0
//   RSI = 50 if both averages are zero
//
// LowestRSI = Lowest RSI over stochasticPeriod
// HighestRSI = Highest RSI over stochasticPeriod
//
// RawStochRSI = 100 * (RSI - LowestRSI) /
//                     (HighestRSI - LowestRSI)
//
// When HighestRSI == LowestRSI:
//   RawStochRSI = 50 (neutral fallback)
//
// K = SMA(RawStochRSI, smoothK)
// D = SMA(K, smoothD)
//
// Current represents smoothed StochRSI %K.
// Previous represents previous smoothed StochRSI %K.
//
// First complete K/D result is ready after
// rsiPeriod + stochasticPeriod + smoothK + smoothD - 1
// valid closing prices.

type StochasticRSIConfig struct {
	RSIPeriod        int
	StochasticPeriod int
	SmoothK          int
	SmoothD          int
	CandleInterval   string
}

type StochasticRSI struct {
	id string

	rsiPeriod        int
	stochasticPeriod int
	smoothK          int
	smoothD          int
	candleInterval   string

	previousClose float64
	hasClose      bool

	changeCount int
	gainSum     float64
	lossSum     float64

	averageGain float64
	averageLoss float64

	rsiValues  []float64
	rawKValues []float64
	kValues    []float64

	lastTimestamp time.Time

	hasResult bool
	lastK     float64
	lastD     float64

	result StochasticRSIResult
}

func NewStochasticRSI(id string, cfg StochasticRSIConfig) (*StochasticRSI, error) {
	if id == "" {
		return nil, fmt.Errorf("stochastic RSI name cannot be empty")
	}

	if cfg.RSIPeriod <= 0 ||
		cfg.StochasticPeriod <= 0 ||
		cfg.SmoothK <= 0 ||
		cfg.SmoothD <= 0 {
		return nil, fmt.Errorf("stochastic RSI periods must be positive")
	}

	return &StochasticRSI{
		id:               id,
		rsiPeriod:        cfg.RSIPeriod,
		stochasticPeriod: cfg.StochasticPeriod,
		smoothK:          cfg.SmoothK,
		smoothD:          cfg.SmoothD,
		candleInterval:   cfg.CandleInterval,
	}, nil
}

func (s *StochasticRSI) Update(candle core.MarketCandle) any {
	s.ingest(candle)
	return s.result
}

func (s *StochasticRSI) Result() any {
	return s.result
}

func (s *StochasticRSI) ingest(candle core.MarketCandle) {
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
		(!s.lastTimestamp.IsZero() &&
			!timestamp.After(s.lastTimestamp)) {
		s.setNotReady(timestamp)
		return
	}

	s.lastTimestamp = timestamp

	// First candle initializes the previous close.
	if !s.hasClose {
		s.previousClose = closePrice
		s.hasClose = true
		s.setNotReady(timestamp)
		return
	}

	change := closePrice - s.previousClose
	s.previousClose = closePrice

	gain := math.Max(change, 0)
	loss := math.Max(-change, 0)

	// Calculate Wilder's RSI.
	if s.changeCount < s.rsiPeriod {
		s.gainSum += gain
		s.lossSum += loss
		s.changeCount++

		if s.changeCount < s.rsiPeriod {
			s.setNotReady(timestamp)
			return
		}

		s.averageGain = s.gainSum / float64(s.rsiPeriod)
		s.averageLoss = s.lossSum / float64(s.rsiPeriod)
	} else {
		n := float64(s.rsiPeriod)

		s.averageGain = (s.averageGain*(n-1) + gain) / n
		s.averageLoss = (s.averageLoss*(n-1) + loss) / n
	}

	var currentRSI float64

	switch {
	case s.averageLoss == 0 && s.averageGain == 0:
		currentRSI = 50

	case s.averageLoss == 0:
		currentRSI = 100

	default:
		rs := s.averageGain / s.averageLoss
		currentRSI = 100 - (100 / (1 + rs))
	}

	if math.IsNaN(currentRSI) || math.IsInf(currentRSI, 0) {
		s.setNotReady(timestamp)
		return
	}

	// Maintain rolling RSI values.
	s.rsiValues = append(s.rsiValues, currentRSI)

	if len(s.rsiValues) > s.stochasticPeriod {
		s.rsiValues = s.rsiValues[1:]
	}

	if len(s.rsiValues) < s.stochasticPeriod {
		s.setNotReady(timestamp)
		return
	}

	lowestRSI := s.rsiValues[0]
	highestRSI := s.rsiValues[0]

	for _, value := range s.rsiValues[1:] {
		if value < lowestRSI {
			lowestRSI = value
		}

		if value > highestRSI {
			highestRSI = value
		}
	}

	// Calculate raw Stochastic RSI on a 0-100 scale.
	rawK := 50.0

	if highestRSI != lowestRSI {
		rawK = 100 * (currentRSI - lowestRSI) / (highestRSI - lowestRSI)
	}

	if math.IsNaN(rawK) || math.IsInf(rawK, 0) {
		s.setNotReady(timestamp)
		return
	}

	s.rawKValues = append(s.rawKValues, rawK)

	if len(s.rawKValues) > s.smoothK {
		s.rawKValues = s.rawKValues[1:]
	}

	if len(s.rawKValues) < s.smoothK {
		s.setNotReady(timestamp)
		return
	}

	// Smooth raw StochRSI using SMA.
	currentK := stochasticRSIAverage(s.rawKValues)

	s.kValues = append(s.kValues, currentK)

	if len(s.kValues) > s.smoothD {
		s.kValues = s.kValues[1:]
	}

	if len(s.kValues) < s.smoothD {
		s.setNotReady(timestamp)
		return
	}

	// Calculate signal line %D.
	currentD := stochasticRSIAverage(s.kValues)

	if math.IsNaN(currentK) ||
		math.IsInf(currentK, 0) ||
		math.IsNaN(currentD) ||
		math.IsInf(currentD, 0) {
		s.setNotReady(timestamp)
		return
	}

	previousK := currentK
	previousD := currentD

	if s.hasResult {
		previousK = s.lastK
		previousD = s.lastD
	}

	s.result = StochasticRSIResult{
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

func (s *StochasticRSI) setNotReady(timestamp time.Time) {
	s.result = StochasticRSIResult{
		IsReady:   false,
		Timestamp: timestamp,
	}
}

func stochasticRSIAverage(values []float64) float64 {
	var sum float64

	for _, value := range values {
		sum += value
	}

	return sum / float64(len(values))
}
