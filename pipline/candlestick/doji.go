package candlestick

import (
	"fmt"
	"math"
	"strings"

	"indicators/core"
)

// describe: Doji - Open and close are nearly equal relative to total candle range.
//
// pipeConfigs: [
//   {
//     type: 'doji',
//     name: 'doji',
//     config: {
//       candleInterval: '1 minutes'
//     }
//   }
// ]
//
// Available Result:
//
//   Current
//   Previous
//   IsReady
//   Timestamp
//
// Entry Condition:
//
//   Pattern Is True:
//     Current == true
//
//   Pattern Becomes True:
//     !Previous && Current
//
// Exit Condition:
//
//   Pattern Is False:
//     Current == false
//
//   Pattern Becomes False:
//     Previous && !Current
//
// Original Pattern Definition:
//
//   A Doji candlestick forms when the opening price
//   and closing price are nearly equal.
//
// Formula:
//
//   Body = abs(Close - Open)
//
//   Range = High - Low
//
//   Doji = Body <= Tolerance * Range
//
// Default Tolerance:
//
//   0.10 (10% of total candle range)
//
// Example:
//
//   Open  = 100
//   High  = 110
//   Low   = 90
//   Close = 101
//
//   Body  = abs(101 - 100) = 1
//   Range = 110 - 90 = 20
//
//   Threshold = 20 * 0.10 = 2
//
//   Body <= Threshold
//   1 <= 2 => Doji = true
//
// Special Case:
//
//   If High == Low, the candle is considered Doji
//   only when Open == Close.
//
// Warm-up:
//
//   1 completed candle.
//
// Candle Aggregation:
//
//   Uses config.candleInterval.
//   Example: "1 minutes", "3 minutes", "5 minutes".
//
// Notes:
//
//   No chartConfig or valuePaths required.
//   Uses core.MarketCandle OHLC data directly.
//
// Reference:
//
//   https://chartschool.stockcharts.com/table-of-contents/chart-analysis/candlestick-charts/candlestick-pattern-dictionary
//
// QuantMan Parity:
//
//   Exact QuantMan Doji tolerance is not verified.
//   The default 10% threshold is an implementation convention.

func NewDoji(id string, cfg map[string]any) (*Doji, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("Doji: name cannot be empty")
	}

	if cfg["candleInterval"] == nil {
		return nil, fmt.Errorf("Doji: candleInterval required")
	}

	if cfg["tolerance"] == nil {
		return nil, fmt.Errorf("Doji: tolerance required")
	}

	candleInterval := cfg["candleInterval"].(string)
	tolerance := cfg["tolerance"].(float64)

	if tolerance == 0 {
		tolerance = 0.10
	}

	if math.IsNaN(tolerance) ||
		math.IsInf(tolerance, 0) ||
		tolerance < 0 ||
		tolerance >= 1 {
		return nil, fmt.Errorf("Doji: tolerance must be between 0 and 1")
	}

	return &Doji{
		id:             id,
		tolerance:      tolerance,
		candleInterval: candleInterval,
	}, nil
}

func (r *Doji) Update(candle core.MarketCandle) any {
	if candle == nil {
		r.result.IsReady = false
		return r.result
	}

	aggregated, ready := r.aggregator.Aggregate(candle)
	if !ready {
		r.result.IsReady = false
		return r.result
	}

	r.ingest(aggregated)

	return r.result
}

func (r *Doji) Result() any {
	return r.result
}

func (r *Doji) ingest(candle core.MarketCandle) {
	if candle == nil {
		r.result.IsReady = false
		return
	}

	timestamp := candle.GetTime()

	if timestamp.IsZero() {
		r.result.IsReady = false
		return
	}

	if !r.lastTimestamp.IsZero() &&
		!timestamp.After(r.lastTimestamp) {
		r.result.IsReady = false
		return
	}

	open := candle.GetOpen()
	high := candle.GetHigh()
	low := candle.GetLow()
	closePrice := candle.GetClose()

	if !dojiValidOHLC(open, high, low, closePrice) {
		r.result.IsReady = false
		return
	}

	body := math.Abs(closePrice - open)
	candleRange := high - low

	detected := false

	if candleRange == 0 {
		detected = body == 0
	} else {
		detected = body <= r.tolerance*candleRange
	}

	previous := detected

	if r.hasResult {
		previous = r.result.Current
	}

	r.result = PatternResult{
		IsReady:   true,
		Current:   detected,
		Previous:  previous,
		Timestamp: timestamp,
	}

	r.lastTimestamp = timestamp
	r.hasResult = true
}

func dojiValidOHLC(open, high, low, closePrice float64) bool {
	values := []float64{
		open,
		high,
		low,
		closePrice,
	}

	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
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

var _ core.Indicator = (*Doji)(nil)
