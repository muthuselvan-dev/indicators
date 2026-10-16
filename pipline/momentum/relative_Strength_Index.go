package momentum

import (
	"fmt"

	"indicators/core"
	"math"
	"time"
)

// describe: Relative Strength Index (RSI)
//
// pipeConfigs: [
//
//	{
//	  type: 'relativeStrengthIndex',
//	  name: 'rsi',
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
//   - Relative Strength Index
//
// Entry Condition:
//
//   RSI Crosses Above 30
//   Previous RSI <= 30 && Current RSI > 30
//
// Exit Condition:
//
//   RSI Crosses Below 30
//   Previous RSI >= 30 && Current RSI < 30
//
// Original Formula (Wilder's RSI):
//
// Change = CurrentClose - PreviousClose
//
// Gain = Max(Change, 0)
// Loss = Max(-Change, 0)
//
// Initial Average Gain = Sum(Gains, period) / period
// Initial Average Loss = Sum(Losses, period) / period
//
// Subsequent Average Gain =
//   ((PreviousAverageGain * (period - 1)) + Gain) / period
//
// Subsequent Average Loss =
//   ((PreviousAverageLoss * (period - 1)) + Loss) / period
//
// RS = AverageGain / AverageLoss
// RSI = 100 - (100 / (1 + RS))
//
// When AverageLoss = 0:
//   RSI = 100 if AverageGain > 0
//   RSI = 50 if both averages are zero
//
// BaseResult.Current represents current RSI.
// BaseResult.Previous represents previous RSI.
//
// With period 14, the first RSI is ready
// after 15 valid closing prices.

type RSI struct {
	id string
	// aggregator *aggregate.Aggregator

	period         int
	candleInterval string

	previousClose float64
	hasClose      bool

	changeCount int
	gainSum     float64
	lossSum     float64

	averageGain float64
	averageLoss float64

	hasResult bool
	lastValue float64

	lastTimestamp time.Time

	result RSIResult
}

func NewRSI(id string, cfg map[string]interface{}) (*RSI, error) {
	if id == "" {
		return nil, fmt.Errorf("RSI name cannot be empty")
	}

	period := 14

	if value, exists := cfg["period"]; exists {
		if p, ok := value.(int); ok {
			period = p
		} else if p, ok := value.(float64); ok {
			period = int(p)
		}
	}

	candleInterval, _ := cfg["candleInterval"].(string)

	var minutes int

	_, err := fmt.Sscanf(candleInterval, "%d minutes", &minutes)
	if err != nil || minutes <= 0 {
		return nil, fmt.Errorf("invalid candleInterval: %s", candleInterval)
	}

	// agg := aggregate.NewAggregator(minutes)

	return &RSI{
		id:             id,
		period:         period,
		candleInterval: candleInterval,
		// aggregator:     agg,
	}, nil
}

func (r *RSI) Update(candle core.MarketCandle) any {

	// aggregated, ready := r.aggregator.Aggregate(candle)

	// if !ready {
	// 	return r.result
	// }

	r.ingest(candle)
	return r.result
}

func (r *RSI) Result() any {
	return r.result
}

func (r *RSI) ingest(candle core.MarketCandle) {
	if candle == nil {
		return
	}

	closePrice := candle.GetClose()
	timestamp := candle.GetTime()

	// Reject invalid data without changing RSI state.
	if timestamp.IsZero() ||
		math.IsNaN(closePrice) ||
		math.IsInf(closePrice, 0) ||
		closePrice <= 0 ||
		(!r.lastTimestamp.IsZero() &&
			!timestamp.After(r.lastTimestamp)) {

		r.result = RSIResult{
			RSIBaseResult: RSIBaseResult{
				IsReady:   false,
				Timestamp: timestamp,
			},
		}
		return
	}

	r.lastTimestamp = timestamp

	// First candle provides the reference close.
	if !r.hasClose {
		r.previousClose = closePrice
		r.hasClose = true

		r.result = RSIResult{
			RSIBaseResult: RSIBaseResult{
				IsReady:   false,
				Timestamp: timestamp,
			},
		}
		return
	}

	change := closePrice - r.previousClose
	r.previousClose = closePrice

	gain := math.Max(change, 0)
	loss := math.Max(-change, 0)

	// Initial RSI seed.
	if r.changeCount < r.period {
		r.gainSum += gain
		r.lossSum += loss
		r.changeCount++

		if r.changeCount < r.period {
			r.result = RSIResult{
				RSIBaseResult: RSIBaseResult{
					IsReady:   false,
					Timestamp: timestamp,
				},
			}
			return
		}

		r.averageGain = r.gainSum / float64(r.period)
		r.averageLoss = r.lossSum / float64(r.period)
	} else {
		// Wilder's smoothing.
		n := float64(r.period)

		r.averageGain =
			(r.averageGain*(n-1) + gain) / n

		r.averageLoss =
			(r.averageLoss*(n-1) + loss) / n
	}

	var currentRSI float64

	switch {
	case r.averageLoss == 0 && r.averageGain == 0:
		currentRSI = 50

	case r.averageLoss == 0:
		currentRSI = 100

	default:
		rs := r.averageGain / r.averageLoss
		currentRSI = 100 - (100 / (1 + rs))
	}

	previousRSI := currentRSI
	if r.hasResult {
		previousRSI = r.lastValue
	}

	r.result = RSIResult{
		RSIBaseResult: RSIBaseResult{
			IsReady:   true,
			Current:   currentRSI,
			Previous:  previousRSI,
			Timestamp: timestamp,
		},
	}

	r.lastValue = currentRSI
	r.hasResult = true
}
