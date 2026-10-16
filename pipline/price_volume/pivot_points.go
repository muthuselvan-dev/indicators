package price_volume

import (
	"fmt"
	"indicators/core"
	"math"
	"strings"
	"time"
)

// describe: Pivot Points
//
// QuantMan Pipe Config:
//   type: "pivotPoints"
//   name: "pivotPoints"
//   config:
//     timeFrame: "day"
//     category: "traditional"
//
// Traditional Pivot Formula (H, L, C = previous day's high, low, close):
//   P  = (H + L + C) / 3
//   R1 = 2P - L          S1 = 2P - H
//   R2 = P + (H - L)     S2 = P - (H - L)
//   R3 = 2P + (H - 2L)   S3 = 2P - (2H - L)
//   R4 = 3P + (H - 3L)   S4 = 3P - (3H - L)
//   R5 = 4P + (H - 4L)   S5 = 4P - (4H - L)
//
// Warm-up:
//   Requires one completed trading day.
//
// Session:
//   Previous completed day's OHLC determines current day's levels.
//   Levels remain frozen throughout the current day.
//   Missing calendar days do not reset the indicator.
//
// Supported:
//   timeFrame: "day"
//   category: "traditional"
//
// Note:
//   Daily grouping uses candle timestamp's timezone.
//   R3-R5 / S3-S5 follow the TradingView traditional formulas.
//   QuantMan exact trade-level parity requires backtest verification.
//
// types.go: PivotPointsResult must contain
//   Resistance3, Resistance4, Resistance5,
//   Support3, Support4, Support5 (float64).

type PivotPointsConfig struct {
	TimeFrame string
	Category  string
}

type PivotPoints struct {
	id  string
	cfg PivotPointsConfig

	lastTimestamp time.Time

	sessionDay   string
	sessionHigh  float64
	sessionLow   float64
	sessionClose float64
	hasSession   bool

	result PivotPointsResult
}

func NewPivotPoints(id string, cfg map[string]any) (*PivotPoints, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("pivotPoints: indicator id required")
	}

	config := PivotPointsConfig{
		TimeFrame: "day",
		Category:  "traditional",
	}

	if value, exists := cfg["timeFrame"]; exists {
		v, ok := value.(string)
		if !ok || strings.TrimSpace(v) == "" {
			return nil, fmt.Errorf("pivotPoints: invalid timeFrame")
		}
		config.TimeFrame = strings.ToLower(strings.TrimSpace(v))
	}

	if value, exists := cfg["category"]; exists {
		v, ok := value.(string)
		if !ok || strings.TrimSpace(v) == "" {
			return nil, fmt.Errorf("pivotPoints: invalid category")
		}
		config.Category = strings.ToLower(strings.TrimSpace(v))
	}

	if config.TimeFrame != "day" {
		return nil, fmt.Errorf(
			"pivotPoints: unsupported timeFrame %q",
			config.TimeFrame,
		)
	}

	if config.Category != "traditional" {
		return nil, fmt.Errorf(
			"pivotPoints: unsupported category %q",
			config.Category,
		)
	}

	return &PivotPoints{
		id:  id,
		cfg: config,
	}, nil
}

func (r *PivotPoints) Update(candle core.MarketCandle) any {
	r.ingest(candle)
	return r.result
}

func (r *PivotPoints) Result() any {
	return r.result
}

func (r *PivotPoints) ingest(candle core.MarketCandle) {
	if !validPivotPointsCandle(candle, r.lastTimestamp) {
		return
	}

	timestamp := candle.GetTime()
	high := candle.GetHigh()
	low := candle.GetLow()
	closePrice := candle.GetClose()

	day := timestamp.Format("2006-01-02")

	r.lastTimestamp = timestamp

	// Initialize first trading day.
	if !r.hasSession {
		r.hasSession = true
		r.sessionDay = day
		r.sessionHigh = high
		r.sessionLow = low
		r.sessionClose = closePrice

		r.result = PivotPointsResult{
			IsReady:   false,
			Timestamp: timestamp,
		}
		return
	}

	// New trading day detected.
	if day != r.sessionDay {
		previousHigh := r.sessionHigh
		previousLow := r.sessionLow
		previousClose := r.sessionClose

		pivot := (previousHigh + previousLow + previousClose) / 3.0

		resistance1 := 2.0*pivot - previousLow
		support1 := 2.0*pivot - previousHigh

		rangeValue := previousHigh - previousLow

		resistance2 := pivot + rangeValue
		support2 := pivot - rangeValue

		resistance3 := 2.0*pivot + (previousHigh - 2.0*previousLow)
		support3 := 2.0*pivot - (2.0*previousHigh - previousLow)

		resistance4 := 3.0*pivot + (previousHigh - 3.0*previousLow)
		support4 := 3.0*pivot - (3.0*previousHigh - previousLow)

		resistance5 := 4.0*pivot + (previousHigh - 4.0*previousLow)
		support5 := 4.0*pivot - (4.0*previousHigh - previousLow)

		if pivotPointsFinite(pivot) &&
			pivotPointsFinite(resistance1) &&
			pivotPointsFinite(support1) &&
			pivotPointsFinite(resistance2) &&
			pivotPointsFinite(support2) &&
			pivotPointsFinite(resistance3) &&
			pivotPointsFinite(support3) &&
			pivotPointsFinite(resistance4) &&
			pivotPointsFinite(support4) &&
			pivotPointsFinite(resistance5) &&
			pivotPointsFinite(support5) {

			r.result = PivotPointsResult{
				IsReady:     true,
				Pivot:       pivot,
				Resistance1: resistance1,
				Support1:    support1,
				Resistance2: resistance2,
				Support2:    support2,
				Resistance3: resistance3,
				Support3:    support3,
				Resistance4: resistance4,
				Support4:    support4,
				Resistance5: resistance5,
				Support5:    support5,
				Timestamp:   timestamp,
			}
		} else {
			r.result = PivotPointsResult{
				IsReady:   false,
				Timestamp: timestamp,
			}
		}

		// Start collecting current trading day's OHLC.
		r.sessionDay = day
		r.sessionHigh = high
		r.sessionLow = low
		r.sessionClose = closePrice

		return
	}

	// Continue collecting current day's OHLC.
	if high > r.sessionHigh {
		r.sessionHigh = high
	}

	if low < r.sessionLow {
		r.sessionLow = low
	}

	r.sessionClose = closePrice

	// Preserve frozen pivot levels.
	r.result.Timestamp = timestamp
}

func pivotPointsFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func validPivotPointsCandle(candle core.MarketCandle, lastTimestamp time.Time) bool {
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
		if !pivotPointsFinite(value) || value <= 0 {
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
