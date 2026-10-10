package volatility

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: Keltner Channel - EMA-based volatility envelope
//
// pipeConfigs: [
//
//	{
//	  type: 'keltnerChannel',
//	  name: 'keltnerChannel',
//	  config: {
//	    emaPeriod: 20,
//	    atrPeriod: 10,
//	    multiplier: 2,
//	    candleInterval: '1 minutes',
//	  }
//	}
//
// ],
//
// Available result:
//
//   - Middle (EMA)
//   - Upper
//   - Lower
//   - Current (Middle)
//   - Previous
//   - IsReady
//   - Timestamp
//
// Entry Condition:
//
//	Close Is Above Upper Band:
//	  Close > Upper
//
//	Close Crosses Above Upper Band:
//	  PreviousClose <= PreviousUpper && Close > Upper
//
// Exit Condition:
//
//	Close Is Below Lower Band:
//	  Close < Lower
//
//	Close Crosses Below Lower Band:
//	  PreviousClose >= PreviousLower && Close < Lower
//
// Original Formula:
//
//	EMA Seed = Sum(First EMA Period Closes) / EMA Period
//
//	EMA Alpha = 2 / (EMA Period + 1)
//
//	EMA = Alpha * CurrentClose +
//	      (1 - Alpha) * PreviousEMA
//
//	TrueRange = Max(
//	  High - Low,
//	  Abs(High - PreviousClose),
//	  Abs(Low - PreviousClose)
//	)
//
//	ATR Seed = Sum(First ATR Period TrueRanges) / ATR Period
//
//	Wilder ATR =
//	  ((PreviousATR * (ATR Period - 1)) + TrueRange) / ATR Period
//
//	Middle = EMA(Close, EMA Period)
//
//	Upper = Middle + Multiplier * ATR
//
//	Lower = Middle - Multiplier * ATR
//
// BaseResult:
//
//	Current:
//	  Latest Middle Band (EMA).
//
//	Previous:
//	  Previous ready Middle Band.
//
//	IsReady:
//	  True when both EMA and ATR are ready.
//
//	Timestamp:
//	  Latest processed valid candle timestamp.
//
// Warmup:
//
//	EMA requires EMA Period valid closing prices.
//	ATR uses High - Low for the first candle's True Range.
//	ATR requires ATR Period valid candles.
//	First ready result requires Max(EMA Period, ATR Period)
//	valid candles.
//
// Reference:
// https://chartschool.stockcharts.com/table-of-contents/technical-indicators-and-overlays/technical-overlays/keltner-channels
//
// Note:
// This implementation uses SMA-seeded EMA and Wilder ATR.
// QuantMan exact calculation parity has not been verified.

type KeltnerChannelConfig struct {
	EMAPeriod      int
	ATRPeriod      int
	Multiplier     float64
	CandleInterval string
}

type KeltnerChannelResult struct {
	BaseResult
	Middle float64
	Upper  float64
	Lower  float64
}

type KeltnerChannel struct {
	id             string
	emaPeriod      int
	atrPeriod      int
	multiplier     float64
	candleInterval string

	closeSum   float64
	closeCount int
	ema        float64

	trSum   float64
	trCount int
	atr     float64

	previousClose float64
	hasClose      bool

	lastTimestamp time.Time

	hasResult bool
	lastValue float64

	result KeltnerChannelResult
}

func NewKeltnerChannel(id string, cfg KeltnerChannelConfig) (*KeltnerChannel, error) {
	if id == "" {
		return nil, fmt.Errorf("Keltner Channel name cannot be empty")
	}

	if cfg.EMAPeriod <= 0 {
		return nil, fmt.Errorf("Keltner Channel EMA period must be positive")
	}

	if cfg.ATRPeriod <= 0 {
		return nil, fmt.Errorf("Keltner Channel ATR period must be positive")
	}

	if cfg.Multiplier <= 0 ||
		math.IsNaN(cfg.Multiplier) ||
		math.IsInf(cfg.Multiplier, 0) {
		return nil, fmt.Errorf(
			"Keltner Channel multiplier must be positive and finite",
		)
	}

	return &KeltnerChannel{
		id:             id,
		emaPeriod:      cfg.EMAPeriod,
		atrPeriod:      cfg.ATRPeriod,
		multiplier:     cfg.Multiplier,
		candleInterval: cfg.CandleInterval,
	}, nil
}

func (k *KeltnerChannel) Update(candle core.MarketCandle) any {
	k.ingest(candle)
	return k.result
}

func (k *KeltnerChannel) Result() any {
	return k.result
}

func (k *KeltnerChannel) ingest(candle core.MarketCandle) {
	
	if !validKeltnerChannelCandle(candle, k.lastTimestamp) {
		return
	}

	timestamp := candle.GetTime()
	high := candle.GetHigh()
	low := candle.GetLow()
	closePrice := candle.GetClose()

	// Calculate True Range.
	trueRange := high - low

	if k.hasClose {
		trueRange = math.Max(
			trueRange,
			math.Max(
				math.Abs(high-k.previousClose),
				math.Abs(low-k.previousClose),
			),
		)
	}

	k.previousClose = closePrice
	k.hasClose = true
	k.lastTimestamp = timestamp

	// EMA calculation with SMA seed.
	if k.closeCount < k.emaPeriod {
		k.closeSum += closePrice
		k.closeCount++

		if k.closeCount == k.emaPeriod {
			k.ema = k.closeSum / float64(k.emaPeriod)
		}
	} else {
		alpha := 2.0 / float64(k.emaPeriod+1)

		k.ema = alpha*closePrice + (1-alpha)*k.ema
	}

	// ATR calculation with Wilder smoothing.
	if k.trCount < k.atrPeriod {
		k.trSum += trueRange
		k.trCount++

		if k.trCount == k.atrPeriod {
			k.atr = k.trSum / float64(k.atrPeriod)
		}
	} else {
		k.atr = (k.atr*float64(k.atrPeriod-1) + trueRange) / float64(k.atrPeriod)
	}

	// Both EMA and ATR must be ready.
	if k.closeCount < k.emaPeriod ||
		k.trCount < k.atrPeriod {
		k.result = KeltnerChannelResult{
			BaseResult: BaseResult{
				IsReady:   false,
				Timestamp: timestamp,
			},
		}
		return
	}

	// Calculate Keltner Channel.
	middle := k.ema
	upper := middle + k.multiplier*k.atr
	lower := middle - k.multiplier*k.atr

	// Reject non-finite calculation results.
	for _, value := range []float64{
		middle,
		upper,
		lower,
	} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return
		}
	}

	// Previous equals Current on first ready candle.
	previous := middle

	if k.hasResult {
		previous = k.lastValue
	}

	k.result = KeltnerChannelResult{
		BaseResult: BaseResult{
			IsReady:   true,
			Current:   middle,
			Previous:  previous,
			Timestamp: timestamp,
		},
		Middle: middle,
		Upper:  upper,
		Lower:  lower,
	}

	k.lastValue = middle
	k.hasResult = true
}

// Own helper function for Keltner Channel.
// No dependency on helpers from other indicator files.
func validKeltnerChannelCandle(candle core.MarketCandle,lastTimestamp time.Time) bool {

	if candle == nil {
		return false
	}

	timestamp := candle.GetTime()

	if timestamp.IsZero() {
		return false
	}

	// Reject duplicate or out-of-order candles.
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
