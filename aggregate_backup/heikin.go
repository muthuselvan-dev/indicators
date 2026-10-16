package aggregate

import (
	"fmt"
	"indicators/core"
)

// HA Close = (O+H+L+C)/4
// HA Open  = (prevHAOpen + prevHAClose)/2   (first candle: (O+C)/2)
// HA High  = max(H, HAOpen, HAClose)
// HA Low   = min(L, HAOpen, HAClose)
type HeikinAshiChart struct {
	agg       *Aggregator
	prevOpen  float64
	prevClose float64
	hasPrev   bool
}

func NewHeikinAshiChart(cfg map[string]any) (*HeikinAshiChart, error) {
	candleInterval := getString(cfg, "candleInterval")
	if candleInterval == "" {
		return nil, fmt.Errorf("heikinashi: candleInterval is required")
	}

	agg, err := NewAggregatorFromString(candleInterval)
	if err != nil {
		return nil, fmt.Errorf("heikinashi: %w", err)
	}
	return &HeikinAshiChart{agg: agg}, nil
}

func (h *HeikinAshiChart) Process(m core.MarketCandle) (core.MarketCandle, bool) {
	c, ok := h.agg.Aggregate(m)
	if !ok {
		return nil, false
	}
	return h.toHeikinAshi(c), true
}

func (h *HeikinAshiChart) toHeikinAshi(c core.MarketCandle) core.Candle {
	o, hi, lo, cl := c.GetOpen(), c.GetHigh(), c.GetLow(), c.GetClose()

	haClose := (o + hi + lo + cl) / 4

	var haOpen float64
	if !h.hasPrev {
		haOpen = (o + cl) / 2
	} else {
		haOpen = (h.prevOpen + h.prevClose) / 2
	}

	haHigh := maxF(hi, maxF(haOpen, haClose))
	haLow := minF(lo, minF(haOpen, haClose))

	h.prevOpen, h.prevClose, h.hasPrev = haOpen, haClose, true

	return core.Candle{
		Time:         c.GetTime(),
		Open:         haOpen,
		High:         haHigh,
		Low:          haLow,
		Close:        haClose,
		Volume:       c.GetVolume(),
		OpenInterest: c.GetOpenInterest(),
	}
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
