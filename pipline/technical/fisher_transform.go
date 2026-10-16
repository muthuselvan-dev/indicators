package technical

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// Indicator Name: Fisher Transform
//
// QuantMan Pipe Config:
//
// {
//     Type: "fisherTransform",
//     Name: "ft",
//     Config: map[string]any{
//         "noOfCandles":    9,
//         "candleInterval": "1 minutes",
//     },
// }
//
// Original Formula:
//
//   MedianPrice = (High + Low) / 2
//
//   Highest = Highest(MedianPrice, Period)
//   Lowest  = Lowest(MedianPrice, Period)
//
//   Normalized = 2 * (
//       (MedianPrice - Lowest) / (Highest - Lowest) - 0.5
//   )
//
//   X = 0.33 * Normalized + 0.67 * PreviousX
//
//   X = Max(-0.999, Min(0.999, X))
//
//   Fisher = 0.5 * Ln((1 + X) / (1 - X)) +
//            0.5 * PreviousFisher
//
// Entry Condition:
//
//   Current Is Above Threshold:
//     Current > Threshold
//
//   Current Crosses Above Threshold:
//     Previous <= Threshold && Current > Threshold
//
// Exit Condition:
//
//   Current Is Below Threshold:
//     Current < Threshold
//
//   Current Crosses Below Threshold:
//     Previous >= Threshold && Current < Threshold
//
// Result:
//
//   ScalarResult:
//     Current  = Latest Fisher Transform value
//     Previous = Previous ready Fisher Transform value
//     IsReady
//     Timestamp
//
// Warmup:
//   First ready result requires Period valid candles.
//
// Important:
//   External QuantMan key: noOfCandles
//   Internal Go field: Period
//
//   When Highest == Lowest, Normalized = 0.
//   Recursive X and Fisher states start at zero.
//
// Tested vs QuantMan (Is Above / Is Below 0, BANKNIFTY 1 min,
// 08 Aug to 28 Aug 2025): 10/13 trades match. The 3 mismatches
// are exits where Fisher is within ~0.02 of the threshold.

type FisherTransformConfig struct {
	Period         int
	CandleInterval string
}

type FisherTransform struct {
	id  string
	cfg FisherTransformConfig

	prices []float64

	x      float64
	fisher float64

	lastTime time.Time

	hasResult bool
	lastValue float64

	result ScalarResult
}

func NewFisherTransform(id string, cfg map[string]any) (*FisherTransform, error) {
	if id == "" {
		return nil, fmt.Errorf("fisherTransform: indicator id is required")
	}

	config := FisherTransformConfig{
		Period:         9,
		CandleInterval: "1 minutes",
	}

	if value, exists := cfg["period"]; exists {
		period, ok := value.(int)
		if !ok {
			return nil, fmt.Errorf("fisherTransform: period must be an integer")
		}
		config.Period = period
	}

	if value, exists := cfg["candleInterval"]; exists {
		interval, ok := value.(string)
		if !ok || interval == "" {
			return nil, fmt.Errorf("fisherTransform: invalid candleInterval")
		}
		config.CandleInterval = interval
	}

	if config.Period < 2 {
		return nil, fmt.Errorf("fisherTransform: period must be at least 2")
	}

	return &FisherTransform{
		id:     id,
		cfg:    config,
		prices: make([]float64, 0, config.Period),
	}, nil
}

func (r *FisherTransform) Update(candle core.MarketCandle) any {
	r.ingest(candle)
	return r.result
}

func (r *FisherTransform) Result() any {
	return r.result
}

func (r *FisherTransform) ingest(candle core.MarketCandle) {
	if !validFisherTransformCandle(candle, r.lastTime) {
		r.result.IsReady = false
		return
	}

	timestamp := candle.GetTime()
	high := candle.GetHigh()
	low := candle.GetLow()

	r.lastTime = timestamp

	// Step 1: Calculate Median Price.
	medianPrice := (high + low) / 2

	// Step 2: Maintain rolling Median Prices.
	r.prices = append(r.prices, medianPrice)

	if len(r.prices) > r.cfg.Period {
		r.prices = r.prices[1:]
	}

	// Step 3: Wait for required candles.
	if len(r.prices) < r.cfg.Period {
		r.result = ScalarResult{
			IsReady:   false,
			Timestamp: timestamp,
		}
		return
	}

	// Step 4: Find Highest and Lowest Median Prices.
	highest := r.prices[0]
	lowest := r.prices[0]

	for i := 1; i < len(r.prices); i++ {
		highest = math.Max(highest, r.prices[i])
		lowest = math.Min(lowest, r.prices[i])
	}

	// Step 5: Normalize current Median Price.
	normalized := 0.0

	if highest > lowest {
		normalized = 2 * ((medianPrice-lowest)/(highest-lowest) - 0.5)
	}

	// Step 6: Apply recursive smoothing.
	x := 0.33*normalized + 0.67*r.x

	// Step 7: Clamp X to avoid logarithm singularity.
	x = math.Max(-0.999, math.Min(0.999, x))

	// Step 8: Calculate Fisher Transform.
	fisher := 0.5*math.Log((1+x)/(1-x)) +
		0.5*r.fisher

	// Step 9: Reject non-finite result.
	if math.IsNaN(fisher) || math.IsInf(fisher, 0) {
		r.result.IsReady = false
		return
	}

	// Step 10: Previous ready Fisher value.
	previous := fisher

	if r.hasResult {
		previous = r.lastValue
	}

	// Step 11: Update result.
	r.result = ScalarResult{
		IsReady:   true,
		Current:   fisher,
		Previous:  previous,
		Timestamp: timestamp,
	}

	// Step 12: Store recursive state.
	r.x = x
	r.fisher = fisher

	r.lastValue = fisher
	r.hasResult = true
}

func validFisherTransformCandle(candle core.MarketCandle, lastTimestamp time.Time) bool {
	if candle == nil {
		return false
	}

	timestamp := candle.GetTime()

	if timestamp.IsZero() {
		return false
	}

	// Reject duplicate or out-of-order candles.
	if !lastTimestamp.IsZero() && !timestamp.After(lastTimestamp) {
		return false
	}

	open := candle.GetOpen()
	high := candle.GetHigh()
	low := candle.GetLow()
	closePrice := candle.GetClose()

	for _, value := range []float64{open, high, low, closePrice} {
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
