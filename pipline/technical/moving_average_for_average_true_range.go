package technical

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// Indicator: Moving Average For ATR
//
// QuantMan Original Config:
//   type: "movingAverageForATR"
//   name: "movingAverageForATR"
//   noOfCandles: 14
//   candleInterval: "1 minutes"
//
// Go Pipe Config:
//   Type: "movingAverageForATR"
//   Name: "movingAverageForATR"
//   Config: map[string]any{
//       "period":         14,
//       "candleInterval": "1 minutes",
//   }
//
// Formula:
//   TR = Max(
//       High - Low,
//       Abs(High - PreviousClose),
//       Abs(Low - PreviousClose),
//   )
//
//   MovingAverageForATR = SMA(TR, Period)
//
// First candle:
//   TR = High - Low
//
// Warmup:
//   Period valid candles.
//
// Entry:
//   Current > Threshold
//   Previous <= Threshold && Current > Threshold
//
// Exit:
//   Current < Threshold
//   Previous >= Threshold && Current < Threshold
//
// Result:
//   Current, Previous, IsReady, Timestamp
//
// Note:
//   This implements SMA of True Range.
//   QuantMan-specific smoothing and parity are unverified.

type MovingAverageForAverageTrueRangeConfig struct {
	Period         int
	CandleInterval string
}

type MovingAverageForAverageTrueRange struct {
	id  string
	cfg MovingAverageForAverageTrueRangeConfig

	trueRanges []float64
	trSum      float64

	previousClose float64
	hasPrevious   bool

	lastTime time.Time

	hasResult bool
	lastValue float64

	result ScalarResult
}

func NewMovingAverageForAverageTrueRange(id string, cfg map[string]any) (*MovingAverageForAverageTrueRange, error) {
	if id == "" {
		return nil, fmt.Errorf("movingAverageForATR: indicator id is required")
	}

	config := MovingAverageForAverageTrueRangeConfig{
		Period:         14,
		CandleInterval: "1 minutes",
	}

	if value, exists := cfg["period"]; exists {
		period, ok := value.(int)
		if !ok {
			return nil, fmt.Errorf("movingAverageForATR: period must be an integer")
		}
		config.Period = period
	}

	if value, exists := cfg["candleInterval"]; exists {
		interval, ok := value.(string)
		if !ok || interval == "" {
			return nil, fmt.Errorf("movingAverageForATR: invalid candleInterval")
		}
		config.CandleInterval = interval
	}

	if config.Period <= 0 {
		return nil, fmt.Errorf("movingAverageForATR: period must be positive")
	}

	return &MovingAverageForAverageTrueRange{
		id:         id,
		cfg:        config,
		trueRanges: make([]float64, 0, config.Period),
	}, nil
}

func (r *MovingAverageForAverageTrueRange) Update(candle core.MarketCandle) any {
	r.ingest(candle)
	return r.result
}

func (r *MovingAverageForAverageTrueRange) Result() any {
	return r.result
}

func (r *MovingAverageForAverageTrueRange) ingest(candle core.MarketCandle) {
	if !validMovingAverageForATRCandle(candle, r.lastTime) {
		r.result.IsReady = false
		return
	}

	timestamp := candle.GetTime()
	high := candle.GetHigh()
	low := candle.GetLow()
	closePrice := candle.GetClose()

	// Step 1: Calculate True Range.
	trueRange := high - low

	if r.hasPrevious {
		trueRange = math.Max(
			high-low,
			math.Max(
				math.Abs(high-r.previousClose),
				math.Abs(low-r.previousClose),
			),
		)
	}

	// Step 2: Maintain rolling True Range window.
	r.trueRanges = append(r.trueRanges, trueRange)
	r.trSum += trueRange

	if len(r.trueRanges) > r.cfg.Period {
		r.trSum -= r.trueRanges[0]
		r.trueRanges = r.trueRanges[1:]
	}

	// Step 3: Update candle state.
	r.previousClose = closePrice
	r.hasPrevious = true
	r.lastTime = timestamp

	// Step 4: Warmup.
	if len(r.trueRanges) < r.cfg.Period {
		r.result = ScalarResult{
			IsReady:   false,
			Timestamp: timestamp,
		}
		return
	}

	// Step 5: SMA of True Range.
	current := r.trSum / float64(r.cfg.Period)

	if math.IsNaN(current) || math.IsInf(current, 0) {
		r.result = ScalarResult{
			IsReady:   false,
			Timestamp: timestamp,
		}
		return
	}

	// Step 6: Previous ready value.
	previous := current

	if r.hasResult {
		previous = r.lastValue
	}

	// Step 7: Store result.
	r.result = ScalarResult{
		IsReady:   true,
		Current:   current,
		Previous:  previous,
		Timestamp: timestamp,
	}

	r.lastValue = current
	r.hasResult = true
}

func validMovingAverageForATRCandle(candle core.MarketCandle, lastTimestamp time.Time) bool {
	if candle == nil {
		return false
	}

	timestamp := candle.GetTime()

	if timestamp.IsZero() {
		return false
	}

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
