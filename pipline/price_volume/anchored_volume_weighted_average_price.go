package price_volume

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: Anchored Volume Weighted Average Price (AVWAP)
//
// pipeConfigs: [
//
//	{
//	  type: 'anchoredVolumeWeightedAveragePrice',
//	  name: 'anchoredVolumeWeightedAveragePrice',
//	  config: {
//	    anchorTime: '<user supplied ISO 8601 timestamp>',
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
//	Close Is Above AVWAP:
//	  Close > Current
//
//	Close Crosses Above AVWAP:
//	  PreviousClose <= Previous && Close > Current
//
// Exit Condition:
//
//	Close Is Below AVWAP:
//	  Close < Current
//
//	Close Crosses Below AVWAP:
//	  PreviousClose >= Previous && Close < Current
//
// Original Formula (anchored candle approximation):
//
//	TypicalPrice = (High + Low + Close) / 3
//
//	AVWAP = Sum(TypicalPrice * Volume, t >= AnchorTime) /
//	        Sum(Volume, t >= AnchorTime)
//
// Anchor Rules:
//
//   - AnchorTime is inclusive.
//   - Candles before AnchorTime do not contribute.
//   - Accumulation does not restart at session boundaries.
//   - Zero-volume candles contribute nothing.
//   - Negative or non-finite volume is rejected.
//   - Duplicate and out-of-order candles are rejected.
//
// Warmup:
//
//   No fixed candle count.
//   The first ready result requires positive accumulated
//   volume at or after AnchorTime.
//
// Previous:
//
//   Previous equals Current on the first ready candle.
//   Afterwards, Previous stores the prior ready AVWAP.
//
// Reference:
// https://chartschool.stockcharts.com/table-of-contents/technical-indicators-and-overlays/technical-overlays/volume-weighted-average-price-vwap
//
// Note:
//
//   Typical-price weighting is a candle-level approximation.
//   QuantMan-specific anchored VWAP parity is not verified.

type AnchoredVolumeWeightedAveragePriceConfig struct {
	AnchorTime     time.Time
	CandleInterval string
}

type AnchoredVolumeWeightedAveragePriceResult struct {
	ScalarResult
}

type AnchoredVolumeWeightedAveragePrice struct {
	id             string
	anchorTime     time.Time
	candleInterval string

	priceVolumeSum float64
	volumeSum      float64

	previousTimestamp time.Time
	previousValue     float64
	hasResult         bool

	result AnchoredVolumeWeightedAveragePriceResult
}

func NewAnchoredVolumeWeightedAveragePrice(
	id string,
	cfg AnchoredVolumeWeightedAveragePriceConfig,
) (*AnchoredVolumeWeightedAveragePrice, error) {
	if id == "" {
		return nil, fmt.Errorf("AVWAP name cannot be empty")
	}

	if cfg.AnchorTime.IsZero() {
		return nil, fmt.Errorf("AVWAP anchor time cannot be empty")
	}

	return &AnchoredVolumeWeightedAveragePrice{
		id:             id,
		anchorTime:     cfg.AnchorTime,
		candleInterval: cfg.CandleInterval,
	}, nil
}

func (a *AnchoredVolumeWeightedAveragePrice) Update(
	candle core.MarketCandle,
) any {
	a.ingest(candle)
	return a.result
}

func (a *AnchoredVolumeWeightedAveragePrice) Result() any {
	return a.result
}

func (a *AnchoredVolumeWeightedAveragePrice) ingest(
	candle core.MarketCandle,
) {
	if !validAnchoredVolumeWeightedAveragePriceCandle(
		candle,
		a.previousTimestamp,
	) {
		return
	}

	timestamp := candle.GetTime()
	high := candle.GetHigh()
	low := candle.GetLow()
	closePrice := candle.GetClose()
	volume := candle.GetVolume()

	a.previousTimestamp = timestamp

	// Ignore candles before the configured anchor.
	if timestamp.Before(a.anchorTime) {
		if !a.hasResult {
			a.result = AnchoredVolumeWeightedAveragePriceResult{
				ScalarResult: ScalarResult{
					IsReady:   false,
					Timestamp: timestamp,
				},
			}
		}
		return
	}

	// Calculate typical price.
	typicalPrice := (high + low + closePrice) / 3.0

	// Validate intermediate values before accumulation.
	weightedPrice := typicalPrice * volume

	if math.IsNaN(typicalPrice) ||
		math.IsInf(typicalPrice, 0) ||
		math.IsNaN(weightedPrice) ||
		math.IsInf(weightedPrice, 0) {
		return
	}

	nextPriceVolumeSum := a.priceVolumeSum + weightedPrice
	nextVolumeSum := a.volumeSum + volume

	if math.IsNaN(nextPriceVolumeSum) ||
		math.IsInf(nextPriceVolumeSum, 0) ||
		math.IsNaN(nextVolumeSum) ||
		math.IsInf(nextVolumeSum, 0) {
		return
	}

	// Calculate AVWAP only after positive accumulated volume.
	if nextVolumeSum <= 0 {
		a.priceVolumeSum = nextPriceVolumeSum
		a.volumeSum = nextVolumeSum

		a.result = AnchoredVolumeWeightedAveragePriceResult{
			ScalarResult: ScalarResult{
				IsReady:   false,
				Timestamp: timestamp,
			},
		}
		return
	}

	current := nextPriceVolumeSum / nextVolumeSum

	if math.IsNaN(current) || math.IsInf(current, 0) {
		return
	}

	// Commit accumulated values.
	a.priceVolumeSum = nextPriceVolumeSum
	a.volumeSum = nextVolumeSum

	// Previous equals Current on first ready candle.
	previous := current

	if a.hasResult {
		previous = a.previousValue
	}

	a.result = AnchoredVolumeWeightedAveragePriceResult{
		ScalarResult: ScalarResult{
			IsReady:   true,
			Current:   current,
			Previous:  previous,
			Timestamp: timestamp,
		},
	}

	a.previousValue = current
	a.hasResult = true
}

// Own validation helper for Anchored VWAP.
// No dependency on helpers from other indicator files.
func validAnchoredVolumeWeightedAveragePriceCandle(
	candle core.MarketCandle,
	lastTimestamp time.Time,
) bool {
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

	// Validate OHLC prices.
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

	// Validate candle price relationships.
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

var _ core.Indicator = (*AnchoredVolumeWeightedAveragePrice)(nil)
