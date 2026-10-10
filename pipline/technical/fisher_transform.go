package technical

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: Fisher Transform
//
// pipeConfigs: [
//   {type: 'fisherTransform', name: 'fisherTransform', config: {period: 10, candleInterval: '1 minutes'}},
// ]
//
// Available result: Current, Previous, IsReady, Timestamp.
// Entry Condition: Is Above threshold: Current > threshold;
//                  Crosses Above threshold: Previous <= threshold && Current > threshold.
// Exit Condition: Is Below threshold: Current < threshold;
//                 Crosses Below threshold: Previous >= threshold && Current < threshold.
//
// Original Formula:
// Median Price = (High + Low)/2.
// Normalize median in rolling period range to [-1,1].
// X = 0.33*2*((Median-Lowest)/(Highest-Lowest)-0.5) + 0.67*previousX.
// Fisher = 0.5*ln((1+X)/(1-X)) + 0.5*previousFisher.
//
// Source: https://www.tradingview.com/support/solutions/43000589141-fisher-transform/
// The examples are strategy rules, not guaranteed QuantMan defaults.
// Warm-up: period candles.

type FisherTransformConfig struct {
	Period         int
	CandleInterval string
}
type FisherTransform struct {
	id             string
	period         int
	candleInterval string
	prices         []float64
	x              float64
	fisher         float64
	lastTime       time.Time
	hasResult      bool
	result         ScalarResult
}

func NewFisherTransform(id string, cfg FisherTransformConfig) (*FisherTransform, error) {
	if id == "" || cfg.Period < 2 {
		return nil, fmt.Errorf("invalid Fisher Transform config")
	}
	return &FisherTransform{id: id, period: cfg.Period, candleInterval: cfg.CandleInterval}, nil
}
func (r *FisherTransform) Update(c core.MarketCandle) any { r.ingest(c); return r.result }
func (r *FisherTransform) Result() any                    { return r.result }
func (r *FisherTransform) ingest(c core.MarketCandle) {
	if c == nil {
		return
	}
	t := c.GetTime()
	h, l := c.GetHigh(), c.GetLow()
	if t.IsZero() || (!r.lastTime.IsZero() && !t.After(r.lastTime)) || math.IsNaN(h) || math.IsNaN(l) || math.IsInf(h, 0) || math.IsInf(l, 0) || h < l || l <= 0 {
		r.result = ScalarResult{Timestamp: t}
		return
	}
	r.lastTime = t
	r.prices = append(r.prices, (h+l)/2)
	if len(r.prices) > r.period {
		r.prices = r.prices[1:]
	}
	if len(r.prices) < r.period {
		r.result = ScalarResult{Timestamp: t}
		return
	}
	lo, hi := r.prices[0], r.prices[0]
	for _, v := range r.prices[1:] {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	normalized := 0.0
	if hi > lo {
		normalized = 2 * ((r.prices[len(r.prices)-1]-lo)/(hi-lo) - 0.5)
	}
	x := 0.33*normalized + 0.67*r.x
	x = math.Max(-0.999, math.Min(0.999, x))
	f := 0.5*math.Log((1+x)/(1-x)) + 0.5*r.fisher
	p := f
	if r.hasResult {
		p = r.result.Current
	}
	r.x = x
	r.fisher = f
	r.result = ScalarResult{IsReady: true, Current: f, Previous: p, Timestamp: t}
	r.hasResult = true
}
