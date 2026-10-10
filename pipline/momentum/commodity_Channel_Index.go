package momentum

import (
	"math"
	"time"

	"indicators/core"
)

// describe: Commodity Channel Index (CCI)
//
// pipeConfigs: [
//
//	{
//	  type: 'commodityChannelIndex',
//	  name: 'commodityChannelIndex',
//	  config: {
//	    period: 20,
//	    candleInterval: '1 minutes',
//	  }
//	}
//
// ],
//
// Available result:
//
//   - Commodity Channel Index
//
// Entry Condition:
//
//   CCI Crosses Above 100
//   Previous CCI <= 100 && Current CCI > 100
//
// Exit Condition:
//
//   CCI Crosses Below -100
//   Previous CCI >= -100 && Current CCI < -100
//
// Original Formula (Commodity Channel Index):
//
// Typical Price = (High + Low + Close) / 3
//
// SMA = Sum(Typical Prices, period) / period
//
// Mean Deviation =
//   Sum(Abs(TypicalPrice - SMA), period) / period
//
// CCI = (CurrentTypicalPrice - SMA) /
//       (0.015 * MeanDeviation)
//
// When MeanDeviation = 0:
//   CCI = 0
//
// BaseResult.Current represents current CCI.
// BaseResult.Previous represents previous CCI.
//
// With period 20, the first CCI is ready
// after 20 valid candles.

type CCI struct {
	id             string
	period         int
	candleInterval string
	prices         []float64
	result         CCIResult
	lastTimestamp  time.Time
}

func NewCCI(id string, cfg map[string]any) (*CCI, error) {
	period, _ := cfg["period"].(int)
	interval, _ := cfg["candleInterval"].(string)

	return &CCI{
		id:             id,
		period:         period,
		candleInterval: interval,
		prices:         make([]float64, 0, period),
	}, nil
}

func (c *CCI) Update(candle core.MarketCandle) any {
	if candle == nil {
		return c.result
	}

	c.ingest(candle)
	return c.result
}

func (c *CCI) Result() any {
	return c.result
}

func (c *CCI) ingest(candle core.MarketCandle) {
	timestamp := candle.GetTime()

	if timestamp.IsZero() ||
		(!c.lastTimestamp.IsZero() &&
			!timestamp.After(c.lastTimestamp)) {
		return
	}

	high := candle.GetHigh()
	low := candle.GetLow()
	closePrice := candle.GetClose()

	if math.IsNaN(high) || math.IsNaN(low) ||
		math.IsNaN(closePrice) ||
		math.IsInf(high, 0) || math.IsInf(low, 0) ||
		math.IsInf(closePrice, 0) ||
		high < low {
		return
	}

	typicalPrice := (high + low + closePrice) / 3

	if math.IsNaN(typicalPrice) ||
		math.IsInf(typicalPrice, 0) {
		return
	}

	c.lastTimestamp = timestamp

	c.prices = append(c.prices, typicalPrice)

	if len(c.prices) > c.period {
		c.prices = c.prices[1:]
	}

	if len(c.prices) < c.period {
		return
	}

	var sum float64

	for _, price := range c.prices {
		sum += price
	}

	sma := sum / float64(c.period)

	var deviation float64

	for _, price := range c.prices {
		deviation += math.Abs(price - sma)
	}

	deviation /= float64(c.period)

	var current float64

	if deviation != 0 {
		current = (typicalPrice - sma) /
			(0.015 * deviation)
	}

	previous := current

	if c.result.IsReady {
		previous = c.result.Current
	}

	c.result = CCIResult{
		IsReady:   true,
		Current:   current,
		Previous:  previous,
		Timestamp: timestamp,
	}
}
