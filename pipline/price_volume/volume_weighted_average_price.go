package price_volume

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: Volume Weighted Average Price (VWAP)
//
// Intraday, candle-based VWAP with daily reset.
//
// pipeConfigs: [
//
//	{
//	  type: 'volumeWeightedAveragePrice',
//	  name: 'volumeWeightedAveragePrice',
//	  config: {
//	    candleInterval: '1 minutes',
//	  },
//	}
//
// ],
//
// Available result:
//
//   - Current
//   - Previous
//   - IsReady
//   - Timestamp
//
// Entry Condition:
//
//	Close Is Above VWAP:
//	  Close > Current
//
//	Close Crosses Above VWAP:
//	  PreviousClose <= Previous && Close > Current
//
// Exit Condition:
//
//	Close Is Below VWAP:
//	  Close < Current
//
//	Close Crosses Below VWAP:
//	  PreviousClose >= Previous && Close < Current
//
// Original Formula:
//
//	TypicalPrice = (High + Low + Close) / 3
//
//	PriceVolume = TypicalPrice * Volume
//
//	VWAP = Sum(PriceVolume) / Sum(Volume)
//
// Session Rules:
//
//   - VWAP resets at each new calendar date.
//   - Date is determined using candle timestamp's location.
//   - Zero-volume candles do not affect accumulated sums.
//   - Duplicate and out-of-order candles are ignored.
//   - Invalid OHLC or volume values are rejected.
//
// Warmup:
//
//   No fixed candle count.
//   First ready result requires positive session volume.
//
// Previous:
//
//   On the first ready candle of each session,
//   Previous equals Current.
//
//   Afterwards, Previous stores the prior ready VWAP
//   within the same session.
//
// Reference:
// https://chartschool.stockcharts.com/table-of-contents/technical-indicators-and-overlays/technical-overlays/volume-weighted-average-price-vwap
//
// Note:
//
//   This is a candle-level approximation of VWAP.
//   QuantMan exact calculation parity is not verified.

type VolumeWeightedAveragePriceConfig struct {
	CandleInterval string
}

type VolumeWeightedAveragePriceResult struct {
	ScalarResult
}

type VolumeWeightedAveragePrice struct {
	id             string
	candleInterval string

	sessionDate string

	priceVolumeSum float64
	volumeSum      float64

	previousTimestamp time.Time

	previousValue float64
	hasResult     bool

	result VolumeWeightedAveragePriceResult
}

func NewVolumeWeightedAveragePrice(
	id string,
	cfg VolumeWeightedAveragePriceConfig,
) (*VolumeWeightedAveragePrice, error) {
	if id == "" {
		return nil, fmt.Errorf("VWAP name cannot be empty")
	}

	return &VolumeWeightedAveragePrice{
		id:             id,
		candleInterval: cfg.CandleInterval,
	}, nil
}

func (v *VolumeWeightedAveragePrice) Update(
	candle core.MarketCandle,
) any {
	v.ingest(candle)
	return v.result
}

func (v *VolumeWeightedAveragePrice) Result() any {
	return v.result
}

func (v *VolumeWeightedAveragePrice) ingest(candle core.MarketCandle) {

	if !validVolumeWeightedAveragePriceCandle(
		candle,
		v.previousTimestamp,
	) {
		return
	}

	timestamp := candle.GetTime()

	high := candle.GetHigh()
	low := candle.GetLow()
	closePrice := candle.GetClose()
	volume := candle.GetVolume()

	sessionDate := timestamp.Format("2006-01-02")

	// Calculate typical price.
	typicalPrice := (high + low + closePrice) / 3.0

	weightedPrice := typicalPrice * volume

	// Validate calculated values.
	if math.IsNaN(typicalPrice) ||
		math.IsInf(typicalPrice, 0) ||
		math.IsNaN(weightedPrice) ||
		math.IsInf(weightedPrice, 0) {
		return
	}

	// Detect new session without changing existing state yet.
	newSession := sessionDate != v.sessionDate

	priceVolumeSum := v.priceVolumeSum
	volumeSum := v.volumeSum

	if newSession {
		priceVolumeSum = 0
		volumeSum = 0
	}

	nextPriceVolumeSum := priceVolumeSum + weightedPrice
	nextVolumeSum := volumeSum + volume

	// Prevent invalid accumulated values.
	if math.IsNaN(nextPriceVolumeSum) ||
		math.IsInf(nextPriceVolumeSum, 0) ||
		math.IsNaN(nextVolumeSum) ||
		math.IsInf(nextVolumeSum, 0) {
		return
	}

	// Commit accepted candle timestamp and session.
	v.previousTimestamp = timestamp

	if newSession {
		v.sessionDate = sessionDate
		v.hasResult = false
		v.previousValue = 0
	}

	v.priceVolumeSum = nextPriceVolumeSum
	v.volumeSum = nextVolumeSum

	// VWAP cannot be calculated before positive volume.
	if v.volumeSum <= 0 {
		v.result = VolumeWeightedAveragePriceResult{
			ScalarResult: ScalarResult{
				IsReady:   false,
				Timestamp: timestamp,
			},
		}
		return
	}

	current := v.priceVolumeSum / v.volumeSum

	if math.IsNaN(current) || math.IsInf(current, 0) {
		v.result = VolumeWeightedAveragePriceResult{
			ScalarResult: ScalarResult{
				IsReady:   false,
				Timestamp: timestamp,
			},
		}
		return
	}

	// Previous equals Current on first ready candle.
	previous := current

	if v.hasResult {
		previous = v.previousValue
	}

	v.result = VolumeWeightedAveragePriceResult{
		ScalarResult: ScalarResult{
			IsReady:   true,
			Current:   current,
			Previous:  previous,
			Timestamp: timestamp,
		},
	}

	v.previousValue = current
	v.hasResult = true
}

// Own validation helper for Volume Weighted Average Price.
//
// This function belongs exclusively to VWAP.
// No dependency on helpers from other indicator files.
func validVolumeWeightedAveragePriceCandle(candle core.MarketCandle, lastTimestamp time.Time) bool {
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
	volume := candle.GetVolume()

	// Validate OHLC.
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

	// Validate volume.
	if math.IsNaN(volume) ||
		math.IsInf(volume, 0) ||
		volume < 0 {
		return false
	}

	// Validate OHLC relationships.
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

var _ core.Indicator = (*VolumeWeightedAveragePrice)(nil)
