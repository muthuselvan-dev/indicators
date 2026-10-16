package technical

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// Indicator Name: Coppock Curve
//
// QuantMan Pipe Config:
//
// {
//     Type: "coppockCurve",
//     Name: "coppockCurve",
//     Config: map[string]any{
//         "candleInterval": "1 minutes",
//         "wmaLength":      10,
//         "longRoCLength":  14,
//         "shortRoCLength": 11,
//     },
// }
//
// Original Formula:
//
//   LongROC = 100 * (Close / Close[LongRoCLength] - 1)
//
//   ShortROC = 100 * (Close / Close[ShortRoCLength] - 1)
//
//   ROCSum = LongROC + ShortROC
//
//   Coppock = WMA(ROCSum, WMALength)
//
//   WMA weights:
//     Oldest = 1
//     Newest = WMALength
//
//   WMA Denominator = WMALength * (WMALength + 1) / 2
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
//     Current  = Latest Coppock value
//     Previous = Previous ready Coppock value
//     IsReady
//     Timestamp
//
// Warmup:
//
//   First ready result requires:
//     Max(LongRoCLength, ShortRoCLength) + WMALength
//     valid candles.
//
// Note:
//   Standard Coppock Curve formula is implemented.
//   QuantMan exact calculation parity is not verified.

type CoppockCurveConfig struct {
	WMALength      int
	LongRoCLength  int
	ShortRoCLength int
	CandleInterval string
}

type CoppockCurve struct {
	id  string
	cfg CoppockCurveConfig

	closes []float64
	roc    []float64

	lastTime time.Time

	hasResult bool
	lastValue float64

	result ScalarResult
}

func NewCoppockCurve(id string, cfg map[string]any) (*CoppockCurve, error) {
	if id == "" {
		return nil, fmt.Errorf("coppockCurve: indicator id is required")
	}

	config := CoppockCurveConfig{
		WMALength:      10,
		LongRoCLength:  14,
		ShortRoCLength: 11,
		CandleInterval: "1 minutes",
	}

	if value, exists := cfg["wmaLength"]; exists {
		length, ok := value.(int)
		if !ok {
			return nil, fmt.Errorf("coppockCurve: wmaLength must be an integer")
		}
		config.WMALength = length
	}

	if value, exists := cfg["longRoCLength"]; exists {
		length, ok := value.(int)
		if !ok {
			return nil, fmt.Errorf("coppockCurve: longRoCLength must be an integer")
		}
		config.LongRoCLength = length
	}

	if value, exists := cfg["shortRoCLength"]; exists {
		length, ok := value.(int)
		if !ok {
			return nil, fmt.Errorf("coppockCurve: shortRoCLength must be an integer")
		}
		config.ShortRoCLength = length
	}

	if value, exists := cfg["candleInterval"]; exists {
		interval, ok := value.(string)
		if !ok || interval == "" {
			return nil, fmt.Errorf("coppockCurve: invalid candleInterval")
		}
		config.CandleInterval = interval
	}

	if config.WMALength <= 0 {
		return nil, fmt.Errorf("coppockCurve: wmaLength must be positive")
	}

	if config.LongRoCLength <= 0 {
		return nil, fmt.Errorf("coppockCurve: longRoCLength must be positive")
	}

	if config.ShortRoCLength <= 0 {
		return nil, fmt.Errorf("coppockCurve: shortRoCLength must be positive")
	}

	maxROC := config.LongRoCLength
	if config.ShortRoCLength > maxROC {
		maxROC = config.ShortRoCLength
	}

	return &CoppockCurve{
		id:     id,
		cfg:    config,
		closes: make([]float64, 0, maxROC+1),
		roc:    make([]float64, 0, config.WMALength),
	}, nil
}

func (r *CoppockCurve) Update(candle core.MarketCandle) any {
	r.ingest(candle)
	return r.result
}

func (r *CoppockCurve) Result() any {
	return r.result
}

func (r *CoppockCurve) ingest(candle core.MarketCandle) {
	if !validCoppockCurveCandle(candle, r.lastTime) {
		r.result.IsReady = false
		return
	}

	timestamp := candle.GetTime()
	closePrice := candle.GetClose()

	r.lastTime = timestamp

	// Step 1: Find maximum ROC length.
	maxROC := r.cfg.LongRoCLength
	if r.cfg.ShortRoCLength > maxROC {
		maxROC = r.cfg.ShortRoCLength
	}

	// Step 2: Maintain rolling closing prices.
	r.closes = append(r.closes, closePrice)

	if len(r.closes) > maxROC+1 {
		r.closes = r.closes[1:]
	}

	// Step 3: Wait for ROC history.
	if len(r.closes) < maxROC+1 {
		r.result = ScalarResult{
			IsReady:   false,
			Timestamp: timestamp,
		}
		return
	}

	// Step 4: Calculate Long and Short ROC.
	index := len(r.closes) - 1

	longROC := 100 * (closePrice/r.closes[index-r.cfg.LongRoCLength] - 1)

	shortROC := 100 * (closePrice/r.closes[index-r.cfg.ShortRoCLength] - 1)

	// Step 5: Sum both ROC values.
	rocSum := longROC + shortROC

	// Step 6: Maintain rolling ROC Sum values.
	r.roc = append(r.roc, rocSum)

	if len(r.roc) > r.cfg.WMALength {
		r.roc = r.roc[1:]
	}

	// Step 7: Wait for WMA history.
	if len(r.roc) < r.cfg.WMALength {
		r.result = ScalarResult{
			IsReady:   false,
			Timestamp: timestamp,
		}
		return
	}

	// Step 8: Calculate Weighted Moving Average.
	var weightedSum float64

	for i, value := range r.roc {
		weightedSum += float64(i+1) * value
	}

	denominator := float64(r.cfg.WMALength) *
		float64(r.cfg.WMALength+1) / 2

	current := weightedSum / denominator

	// Step 9: Validate calculated result.
	if math.IsNaN(current) || math.IsInf(current, 0) {
		r.result.IsReady = false
		return
	}

	// Step 10: Previous ready Coppock value.
	previous := current

	if r.hasResult {
		previous = r.lastValue
	}

	// Step 11: Update result.
	r.result = ScalarResult{
		IsReady:   true,
		Current:   current,
		Previous:  previous,
		Timestamp: timestamp,
	}

	r.lastValue = current
	r.hasResult = true
}

func validCoppockCurveCandle(candle core.MarketCandle, lastTimestamp time.Time) bool {
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
