package volatility

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: Donchian Channel - rolling highest high and lowest low
//
// pipeConfigs: [
//
//	{
//	  type: 'donChianChannel',
//	  name: 'donChianChannel',
//	  config: {
//	    noOfCandles: 20,
//	    candleInterval: '1 minutes',
//	  }
//	}
//
// ],
//
// Available result:
//
//   - Upper
//   - Lower
//   - Middle
//   - Current (Middle)
//   - Previous
//   - IsReady
//   - Timestamp
//
// Entry Condition:
//
//	Close Is Above Previous Upper Band:
//	  Close > PreviousUpper
//
//	Close Crosses Above Previous Upper Band:
//	  PreviousClose <= PreviousUpper && Close > PreviousUpper
//
// Exit Condition:
//
//	Close Is Below Previous Lower Band:
//	  Close < PreviousLower
//
//	Close Crosses Below Previous Lower Band:
//	  PreviousClose >= PreviousLower && Close < PreviousLower
//
// Original Formula:
//
//	Upper = HighestHigh(Last N Candles)
//
//	Lower = LowestLow(Last N Candles)
//
//	Middle = (Upper + Lower) / 2
//
// BaseResult:
//
//	Current:
//	  Latest Middle Channel value.
//
//	Previous:
//	  Previous ready Middle Channel value.
//
//	IsReady:
//	  True after N valid candles.
//
//	Timestamp:
//	  Latest processed valid candle timestamp.
//
// Warmup:
//
//	First ready result requires N valid candles.
//
// Important:
//
//	The current candle is included in the channel calculation.
//	For breakout strategies, compare the current price with
//	the previous completed channel to avoid look-ahead bias.
//
// Reference:
// https://www.investopedia.com/terms/d/donchianchannels.asp
//
// Note:
// QuantMan exact calculation parity has not been verified.

type DonchianChannelConfig struct {
	NoOfCandles    int
	CandleInterval string
}

type DonchianChannel struct {
	id             string
	period         int
	candleInterval string

	highs []float64
	lows  []float64

	lastTimestamp time.Time

	hasResult bool
	lastValue float64

	result DonchianChannelResult
}

func NewDonchianChannel(id string, cfg DonchianChannelConfig) (*DonchianChannel, error) {
	if id == "" {
		return nil, fmt.Errorf("Donchian Channel name cannot be empty")
	}

	if cfg.NoOfCandles <= 0 {
		return nil, fmt.Errorf("Donchian Channel period must be positive")
	}

	return &DonchianChannel{
		id:             id,
		period:         cfg.NoOfCandles,
		candleInterval: cfg.CandleInterval,
	}, nil
}

func (d *DonchianChannel) Update(candle core.MarketCandle) any {
	d.ingest(candle)
	return d.result
}

func (d *DonchianChannel) Result() any {
	return d.result
}

func (d *DonchianChannel) ingest(candle core.MarketCandle) {
	if !validDonchianChannelCandle(candle, d.lastTimestamp) {
		return
	}

	timestamp := candle.GetTime()
	high := candle.GetHigh()
	low := candle.GetLow()

	d.lastTimestamp = timestamp

	// Maintain rolling High and Low values.
	d.highs = append(d.highs, high)
	d.lows = append(d.lows, low)

	if len(d.highs) > d.period {
		d.highs = d.highs[1:]
		d.lows = d.lows[1:]
	}

	// Wait until N valid candles.
	if len(d.highs) < d.period {
		d.result = DonchianChannelResult{
			BaseResult: BaseResult{
				IsReady:   false,
				Timestamp: timestamp,
			},
		}
		return
	}

	// Find highest high and lowest low.
	upper := d.highs[0]
	lower := d.lows[0]

	for i := 1; i < len(d.highs); i++ {
		upper = math.Max(upper, d.highs[i])
		lower = math.Min(lower, d.lows[i])
	}

	// Calculate middle channel.
	middle := (upper + lower) / 2

	// Previous equals Current on first ready candle.
	previous := middle

	if d.hasResult {
		previous = d.lastValue
	}

	d.result = DonchianChannelResult{
		BaseResult: BaseResult{
			IsReady:   true,
			Current:   middle,
			Previous:  previous,
			Timestamp: timestamp,
		},
		Upper:  upper,
		Lower:  lower,
		Middle: middle,
	}

	d.lastValue = middle
	d.hasResult = true
}

// Own validation helper for Donchian Channel.
// No dependency on other indicator helper functions.
func validDonchianChannelCandle(candle core.MarketCandle, lastTimestamp time.Time) bool {

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
