package price_volume

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: Central Pivot Range (CPR)
//
// Calculates Pivot, Top Central and Bottom Central
// using the previous completed calendar day's OHLC.
//
// pipeConfigs: [
//
//	{
//	  type: 'centralPivotRange',
//	  name: 'centralPivotRange',
//	  config: {
//	    candleInterval: '1 minutes',
//	  }
//	}
//
// ],
//
// Available result:
//
//   - Pivot
//   - Top
//   - Bottom
//   - IsReady
//   - Timestamp
//
// Entry Condition:
//
//	Price Is Above Pivot:
//	  CurrentClose > Pivot
//
//	Price Crosses Above Pivot:
//	  PreviousClose <= Pivot && CurrentClose > Pivot
//
// Exit Condition:
//
//	Price Is Below Pivot:
//	  CurrentClose < Pivot
//
//	Price Crosses Below Pivot:
//	  PreviousClose >= Pivot && CurrentClose < Pivot
//
// Original Formula:
//
//	Pivot = (PreviousHigh + PreviousLow + PreviousClose) / 3
//
//	BC = (PreviousHigh + PreviousLow) / 2
//
//	TC = (2 * Pivot) - BC
//
//	Top = Max(TC, BC)
//
//	Bottom = Min(TC, BC)
//
// Warmup:
//
//	Requires one completed calendar day.
//
// Session Handling:
//
//   - Calendar date is determined using candle timestamp's timezone.
//   - Previous completed day's CPR is frozen throughout the current day.
//   - Current day's High, Low and Close are accumulated separately.
//   - New CPR levels are calculated on the first valid candle of a new day.
//   - Invalid and out-of-order candles are ignored.
//   - Missing calendar days do not automatically reset the indicator.
//
// Reference:
// https://chartschool.stockcharts.com/table-of-contents/technical-indicators-and-overlays/technical-overlays/pivot-points
//
// Note:
// Exchange-session handling and QuantMan parity are not verified.

type CentralPivotRangeConfig struct {
	CandleInterval string
}

type CentralPivotRangeResult struct {
	IsReady   bool
	Pivot     float64
	Top       float64
	Bottom    float64
	Timestamp time.Time
}

type CentralPivotRange struct {
	id             string
	candleInterval string

	lastTimestamp time.Time

	day string

	high  float64
	low   float64
	close float64

	hasDay bool

	result CentralPivotRangeResult
}

func NewCentralPivotRange(
	id string,
	cfg CentralPivotRangeConfig,
) (*CentralPivotRange, error) {
	if id == "" {
		return nil, fmt.Errorf("Central Pivot Range name cannot be empty")
	}

	return &CentralPivotRange{
		id:             id,
		candleInterval: cfg.CandleInterval,
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

	// Initialize the first observed calendar day.
	if !r.hasDay {
		r.hasDay = true
		r.day = day

		r.high = high
		r.low = low
		r.close = closePrice

		r.result = CentralPivotRangeResult{
			IsReady:   false,
			Timestamp: timestamp,
		}
		return
	}

	// New calendar day detected.
	if day != r.day {
		// Calculate CPR from the completed previous day.
		pivot := (r.high + r.low + r.close) / 3.0

		bottomCentral := (r.high + r.low) / 2.0

		topCentral := 2.0*pivot - bottomCentral

		top := math.Max(topCentral, bottomCentral)
		bottom := math.Min(topCentral, bottomCentral)

		// Commit the completed day's CPR.
		if !math.IsNaN(pivot) &&
			!math.IsInf(pivot, 0) &&
			!math.IsNaN(top) &&
			!math.IsInf(top, 0) &&
			!math.IsNaN(bottom) &&
			!math.IsInf(bottom, 0) {
			r.result = CentralPivotRangeResult{
				IsReady:   true,
				Pivot:     pivot,
				Top:       top,
				Bottom:    bottom,
				Timestamp: timestamp,
			}
		}

		// Start accumulating the new day's OHLC.
		r.day = day
		r.high = high
		r.low = low
		r.close = closePrice

		return
	}

	// Continue accumulating current day's OHLC.
	if high > r.high {
		r.high = high
	}

	if low < r.low {
		r.low = low
	}

	r.close = closePrice

	// Preserve frozen CPR levels for the current session.
	r.result.Timestamp = timestamp
}

// Own validation helper for Central Pivot Range.
// No dependency on other indicator files.

func validCentralPivotRangeCandle(candle core.MarketCandle, lastTimestamp time.Time) bool {

	if candle == nil {
		return false
	}

	timestamp := candle.GetTime()

	if timestamp.IsZero() {
		return false
	}

	// Reject duplicate and out-of-order candles.
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

var _ core.Indicator = (*CentralPivotRange)(nil)
