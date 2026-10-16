package price_volume

import (
	"fmt"
	"indicators/core"
	"math"
	"strings"
	"time"
)

// describe: Central Pivot Range (CPR)
//
// QuantMan Pipe Config:
//   type: "centralPivotRange"
//   name: "cpr"
//   config:
//     timeFrame:       "day"
//     category:        "traditional"
//     narrowRange:     0.2
//     moderatelyRange: 0.5
//     wideRange:       0.5
//
// Traditional CPR Formula:
//   Pivot = (PreviousHigh + PreviousLow + PreviousClose) / 3
//   BC    = (PreviousHigh + PreviousLow) / 2
//   TC    = 2*Pivot - BC
//   Top   = Max(TC, BC)
//   Bottom = Min(TC, BC)
//
// Warm-up:
//   Requires one completed trading day.
//
// Session Behavior:
//   Previous completed day's OHLC determines today's CPR.
//   CPR levels remain unchanged throughout the current day.
//   Missing calendar days do not reset the calculation.
//
// Supported:
//   timeFrame: "day"
//   category:  "traditional"
//
// Note:
//   Range thresholds are retained as configuration.
//   QuantMan classification semantics are not verified.
//   Exact QuantMan parity requires backtest verification.

type CentralPivotRangeConfig struct {
	TimeFrame       string
	Category        string
	NarrowRange     float64
	ModeratelyRange float64
	WideRange       float64
}

type CentralPivotRange struct {
	id  string
	cfg CentralPivotRangeConfig

	lastTimestamp time.Time

	sessionDay   string
	sessionHigh  float64
	sessionLow   float64
	sessionClose float64
	hasSession   bool

	result CentralPivotRangeResult
}

func NewCentralPivotRange(id string, cfg map[string]any) (*CentralPivotRange, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("centralPivotRange: indicator id required")
	}

	config := CentralPivotRangeConfig{
		TimeFrame:       "day",
		Category:        "traditional",
		NarrowRange:     0.2,
		ModeratelyRange: 0.5,
		WideRange:       0.5,
	}

	if value, exists := cfg["timeFrame"]; exists {
		v, ok := value.(string)
		if !ok || strings.TrimSpace(v) == "" {
			return nil, fmt.Errorf("centralPivotRange: invalid timeFrame")
		}
		config.TimeFrame = strings.ToLower(strings.TrimSpace(v))
	}

	if value, exists := cfg["category"]; exists {
		v, ok := value.(string)
		if !ok || strings.TrimSpace(v) == "" {
			return nil, fmt.Errorf("centralPivotRange: invalid category")
		}
		config.Category = strings.ToLower(strings.TrimSpace(v))
	}

	if value, exists := cfg["narrowRange"]; exists {
		v, err := cprNumericValue(value)
		if err != nil {
			return nil, fmt.Errorf("centralPivotRange: narrowRange: %w", err)
		}
		config.NarrowRange = v
	}

	if value, exists := cfg["moderatelyRange"]; exists {
		v, err := cprNumericValue(value)
		if err != nil {
			return nil, fmt.Errorf("centralPivotRange: moderatelyRange: %w", err)
		}
		config.ModeratelyRange = v
	}

	if value, exists := cfg["wideRange"]; exists {
		v, err := cprNumericValue(value)
		if err != nil {
			return nil, fmt.Errorf("centralPivotRange: wideRange: %w", err)
		}
		config.WideRange = v
	}

	if config.TimeFrame != "day" {
		return nil, fmt.Errorf(
			"centralPivotRange: unsupported timeFrame %q",
			config.TimeFrame,
		)
	}

	if config.Category != "traditional" {
		return nil, fmt.Errorf(
			"centralPivotRange: unsupported category %q",
			config.Category,
		)
	}

	for name, value := range map[string]float64{
		"narrowRange":     config.NarrowRange,
		"moderatelyRange": config.ModeratelyRange,
		"wideRange":       config.WideRange,
	} {
		if math.IsNaN(value) ||
			math.IsInf(value, 0) ||
			value < 0 {
			return nil, fmt.Errorf(
				"centralPivotRange: invalid %s",
				name,
			)
		}
	}

	return &CentralPivotRange{
		id:  id,
		cfg: config,
	}, nil
}

func (r *CentralPivotRange) Update(candle core.MarketCandle) any {
	r.ingest(candle)
	return r.result
}

func (r *CentralPivotRange) Result() any {
	return r.result
}

func (r *CentralPivotRange) ingest(candle core.MarketCandle) {
	if !validCentralPivotRangeCandle(candle, r.lastTimestamp) {
		return
	}

	timestamp := candle.GetTime()
	high := candle.GetHigh()
	low := candle.GetLow()
	closePrice := candle.GetClose()

	day := timestamp.Format("2006-01-02")

	r.lastTimestamp = timestamp

	// Step 1: Initialize first trading day.
	if !r.hasSession {
		r.hasSession = true
		r.sessionDay = day
		r.sessionHigh = high
		r.sessionLow = low
		r.sessionClose = closePrice

		r.result = CentralPivotRangeResult{
			IsReady:   false,
			Timestamp: timestamp,
		}
		return
	}

	// Step 2: Detect new trading day.
	if day != r.sessionDay {
		previousHigh := r.sessionHigh
		previousLow := r.sessionLow
		previousClose := r.sessionClose

		// Step 3: Traditional CPR calculation.
		pivot := (previousHigh +
			previousLow +
			previousClose) / 3.0

		bottomCentral := (previousHigh +
			previousLow) / 2.0

		topCentral := 2.0*pivot - bottomCentral

		top := math.Max(topCentral, bottomCentral)
		bottom := math.Min(topCentral, bottomCentral)

		// Step 4: Publish frozen CPR levels.
		if cprFinite(pivot) &&
			cprFinite(top) &&
			cprFinite(bottom) {

			r.result = CentralPivotRangeResult{
				IsReady:   true,
				Pivot:     pivot,
				Top:       top,
				Bottom:    bottom,
				Timestamp: timestamp,
			}
		} else {
			r.result = CentralPivotRangeResult{
				IsReady:   false,
				Timestamp: timestamp,
			}
		}

		// Step 5: Initialize current trading day.
		r.sessionDay = day
		r.sessionHigh = high
		r.sessionLow = low
		r.sessionClose = closePrice

		return
	}

	// Step 6: Accumulate today's OHLC.
	if high > r.sessionHigh {
		r.sessionHigh = high
	}

	if low < r.sessionLow {
		r.sessionLow = low
	}

	r.sessionClose = closePrice

	// Step 7: Keep CPR values frozen.
	r.result.Timestamp = timestamp
}

func cprNumericValue(value any) (float64, error) {
	switch v := value.(type) {
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case float32:
		return float64(v), nil
	case float64:
		return v, nil
	default:
		return 0, fmt.Errorf("expected numeric value, got %T", value)
	}
}

func cprFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func validCentralPivotRangeCandle(candle core.MarketCandle, lastTimestamp time.Time) bool {
	if candle == nil {
		return false
	}

	timestamp := candle.GetTime()

	if timestamp.IsZero() {
		return false
	}

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
		if !cprFinite(value) || value <= 0 {
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
