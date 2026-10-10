package candlestick

import (
	"indicators/core"
	"math"
	"time"
)

type PatternResult struct {
	IsReady   bool
	Current   bool
	Previous  bool
	Timestamp time.Time
}
type patternCandle struct{ open, high, low, close float64 }

func (c patternCandle) bull() bool          { return c.close > c.open }
func (c patternCandle) bear() bool          { return c.close < c.open }
func (c patternCandle) body() float64       { return math.Abs(c.close - c.open) }
func (c patternCandle) upper() float64      { return c.high - math.Max(c.open, c.close) }
func (c patternCandle) lower() float64      { return math.Min(c.open, c.close) - c.low }
func (c patternCandle) rangeValue() float64 { return c.high - c.low }
func candleFrom(c core.MarketCandle) patternCandle {
	return patternCandle{c.GetOpen(), c.GetHigh(), c.GetLow(), c.GetClose()}
}
func valid(c core.MarketCandle, t time.Time) bool {
	if c == nil || c.GetTime().IsZero() || (!t.IsZero() && !c.GetTime().After(t)) {
		return false
	}
	o, h, l, cl := c.GetOpen(), c.GetHigh(), c.GetLow(), c.GetClose()
	for _, v := range []float64{o, h, l, cl} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
			return false
		}
	}
	return h >= l && h >= math.Max(o, cl) && l <= math.Min(o, cl) && h > l
}
func max3(a, b, c float64) float64 { return math.Max(a, math.Max(b, c)) }
func min3(a, b, c float64) float64 { return math.Min(a, math.Min(b, c)) }
