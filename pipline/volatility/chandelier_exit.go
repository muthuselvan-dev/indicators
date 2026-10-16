package volatility

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// Indicator Name: Chandelier Exit
//
// Pipe Config:
//
// {
//     Type: "chandelierExit",
//     Name: "chandelierExit",
//     Config: map[string]any{
//         "period":         22,
//         "multiplier":     3.0,
//         "useClosePrice":  true,
//         "candleInterval": "1 minutes",
//     },
// }
//
// Entry Condition (QuantMan):
//   Current Close Is Above Chandelier Exit Long Stop:
//     Close > Current
//
// Exit Condition (QuantMan):
//   Current Close Is Below Chandelier Exit Long Stop:
//     Close < Current
//
// Formula:
//
//   TrueRange = Max(
//       High - Low,
//       Abs(High - PreviousClose),
//       Abs(Low - PreviousClose)
//   )
//
//   FirstATR = Sum(First Period TrueRanges) / Period
//
//   NextATR =
//       (PreviousATR * (Period - 1) + TrueRange) / Period
//
//   If UseClosePrice = true:
//       HighestValue = HighestClose(Period)
//   If UseClosePrice = false:
//       HighestValue = HighestHigh(Period)
//
//   RawLongStop = HighestValue - Multiplier * ATR
//
//   Long Stop (ratchet):
//       If PreviousClose > PreviousLongStop:
//           LongStop = Max(RawLongStop, PreviousLongStop)
//       Else:
//           LongStop = RawLongStop
//
//   The ratchet means the Long Stop never moves down while price stays
//   above it. Verified: 13/13 QuantMan trades match (BANKNIFTY 1 minute,
//   2025-08-08 to 2025-08-28, Is Above / Is Below the Long Stop).
//
// Result:
//   ScalarResult:
//     Current  = LongStop
//     Previous = Previous ready LongStop
//     IsReady
//     Timestamp
//
// Warmup:
//   First candle establishes PreviousClose.
//   Next Period candles initialize ATR.
//   First ready result requires Period + 1 valid candles.
//
// Note:
//   The Short Stop is not exposed by ScalarResult.

type ChandelierExitConfig struct {
	Period         int
	Multiplier     float64
	UseClosePrice  bool
	CandleInterval string
}

type ChandelierExit struct {
	id  string
	cfg ChandelierExitConfig

	highs []float64
	lows  []float64

	atr     float64
	trSum   float64
	trCount int

	lastClose float64
	hasClose  bool

	lastTime time.Time

	// Previous ready (ratcheted) Long Stop.
	hasResult bool
	lastValue float64

	result ScalarResult
}

func NewChandelierExit(id string, cfg map[string]any) (*ChandelierExit, error) {
	if id == "" {
		return nil, fmt.Errorf("chandelierExit: indicator id is required")
	}

	config := ChandelierExitConfig{
		Period:         22,
		Multiplier:     3.0,
		UseClosePrice:  true,
		CandleInterval: "1 minutes",
	}

	if value, exists := cfg["period"]; exists {
		period, ok := value.(int)
		if !ok {
			return nil, fmt.Errorf("chandelierExit: period must be an integer")
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
			return nil, fmt.Errorf("chandelierExit: multiplier must be numeric")
		}
	}

	if value, exists := cfg["useClosePrice"]; exists {
		useClosePrice, ok := value.(bool)
		if !ok {
			return nil, fmt.Errorf("chandelierExit: useClosePrice must be boolean")
		}
		config.UseClosePrice = useClosePrice
	}

	if value, exists := cfg["candleInterval"]; exists {
		interval, ok := value.(string)
		if !ok || interval == "" {
			return nil, fmt.Errorf("chandelierExit: invalid candleInterval")
		}
		config.CandleInterval = interval
	}

	if config.Period <= 0 {
		return nil, fmt.Errorf("chandelierExit: period must be positive")
	}

	if math.IsNaN(config.Multiplier) ||
		math.IsInf(config.Multiplier, 0) ||
		config.Multiplier <= 0 {
		return nil, fmt.Errorf("chandelierExit: multiplier must be positive and finite")
	}

	return &ChandelierExit{
		id:    id,
		cfg:   config,
		highs: make([]float64, 0, config.Period),
		lows:  make([]float64, 0, config.Period),
	}, nil
}

func (r *ChandelierExit) Update(candle core.MarketCandle) any {
	r.ingest(candle)
	return r.result
}

func (r *ChandelierExit) Result() any {
	return r.result
}

func (r *ChandelierExit) ingest(candle core.MarketCandle) {
	if !validChandelierExitCandle(candle, r.lastTime) {
		r.result.IsReady = false
		return
	}

	timestamp := candle.GetTime()
	high := candle.GetHigh()
	low := candle.GetLow()
	closePrice := candle.GetClose()

	highestSource := high
	lowestSource := low

	if r.cfg.UseClosePrice {
		highestSource = closePrice
		lowestSource = closePrice
	}

	// First candle establishes PreviousClose.
	if !r.hasClose {
		r.lastClose = closePrice
		r.hasClose = true
		r.lastTime = timestamp

		r.highs = append(r.highs, highestSource)
		r.lows = append(r.lows, lowestSource)

		r.result = ScalarResult{
			IsReady:   false,
			Timestamp: timestamp,
		}
		return
	}

	// Step 1: True Range (uses the previous close).
	previousClose := r.lastClose

	trueRange := math.Max(
		high-low,
		math.Max(
			math.Abs(high-previousClose),
			math.Abs(low-previousClose),
		),
	)

	r.lastClose = closePrice
	r.lastTime = timestamp

	// Step 2: Maintain rolling price sources.
	r.highs = append(r.highs, highestSource)
	r.lows = append(r.lows, lowestSource)

	if len(r.highs) > r.cfg.Period {
		r.highs = r.highs[1:]
		r.lows = r.lows[1:]
	}

	// Step 3: ATR initialization and Wilder smoothing.
	if r.trCount < r.cfg.Period {
		r.trSum += trueRange
		r.trCount++

		if r.trCount == r.cfg.Period {
			r.atr = r.trSum / float64(r.cfg.Period)
		}
	} else {
		r.atr = (r.atr*float64(r.cfg.Period-1) + trueRange) / float64(r.cfg.Period)
	}

	// Wait for ATR and rolling price window.
	if r.trCount < r.cfg.Period || len(r.highs) < r.cfg.Period {
		r.result = ScalarResult{
			IsReady:   false,
			Timestamp: timestamp,
		}
		return
	}

	// Step 4: Highest value over the window.
	highestValue := r.highs[0]

	for i := 1; i < len(r.highs); i++ {
		if r.highs[i] > highestValue {
			highestValue = r.highs[i]
		}
	}

	// Step 5: Long Stop with ratchet.
	longStop := highestValue - r.cfg.Multiplier*r.atr

	if math.IsNaN(longStop) || math.IsInf(longStop, 0) {
		r.result.IsReady = false
		return
	}

	// The Long Stop never moves down while the previous close was above it.
	if r.hasResult && previousClose > r.lastValue {
		longStop = math.Max(longStop, r.lastValue)
	}

	previous := longStop

	if r.hasResult {
		previous = r.lastValue
	}

	r.result = ScalarResult{
		IsReady:   true,
		Current:   longStop,
		Previous:  previous,
		Timestamp: timestamp,
	}

	r.lastValue = longStop
	r.hasResult = true
}

func validChandelierExitCandle(candle core.MarketCandle, lastTimestamp time.Time) bool {
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
		if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
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
