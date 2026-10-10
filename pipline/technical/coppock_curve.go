package technical

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: Coppock Curve
//
// pipeConfigs: [
//   {type: 'coppockCurve', name: 'coppockCurve', config: {longROC: 14, shortROC: 11, wmaPeriod: 10, candleInterval: '1 minutes'}},
// ]
//
// Available result: Current, Previous, IsReady, Timestamp.
// Entry Condition: Is Above threshold: Current > threshold;
//                  Crosses Above threshold: Previous <= threshold && Current > threshold.
// Exit Condition: Is Below threshold: Current < threshold;
//                 Crosses Below threshold: Previous >= threshold && Current < threshold.
//
// Original Formula:
// ROC(n) = 100 * (Close / Close[n] - 1).
// Coppock = WMA(ROC(longROC) + ROC(shortROC), wmaPeriod).
// WMA weights oldest=1, newest=wmaPeriod.
//
// Source: https://chartschool.stockcharts.com/table-of-contents/technical-indicators-and-overlays/technical-indicators/coppock-curve
// The examples are strategy rules, not guaranteed QuantMan defaults.
// Warm-up: max(longROC,shortROC)+wmaPeriod candles.

type CoppockCurveConfig struct {
	LongROC        int
	ShortROC       int
	WMAPeriod      int
	CandleInterval string
}
type CoppockCurve struct {
	id                           string
	longROC, shortROC, wmaPeriod int
	candleInterval               string
	closes, roc                  []float64
	lastTime                     time.Time
	hasResult                    bool
	result                       ScalarResult
}

func NewCoppockCurve(id string, cfg CoppockCurveConfig) (*CoppockCurve, error) {
	if id == "" || cfg.LongROC < 1 || cfg.ShortROC < 1 || cfg.WMAPeriod < 1 {
		return nil, fmt.Errorf("invalid Coppock Curve config")
	}
	return &CoppockCurve{id: id, longROC: cfg.LongROC, shortROC: cfg.ShortROC, wmaPeriod: cfg.WMAPeriod, candleInterval: cfg.CandleInterval}, nil
}
func (r *CoppockCurve) Update(c core.MarketCandle) any { r.ingest(c); return r.result }
func (r *CoppockCurve) Result() any                    { return r.result }
func (r *CoppockCurve) ingest(c core.MarketCandle) {
	if c == nil {
		return
	}
	t := c.GetTime()
	v := c.GetClose()
	if t.IsZero() || (!r.lastTime.IsZero() && !t.After(r.lastTime)) || v <= 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		r.result = ScalarResult{Timestamp: t}
		return
	}
	r.lastTime = t
	r.closes = append(r.closes, v)
	maxROC := r.longROC
	if r.shortROC > maxROC {
		maxROC = r.shortROC
	}
	if len(r.closes) > maxROC+1 {
		r.closes = r.closes[1:]
	}
	if len(r.closes) < maxROC+1 {
		r.result = ScalarResult{Timestamp: t}
		return
	}
	idx := len(r.closes) - 1
	value := 100*(v/r.closes[idx-r.longROC]-1) + 100*(v/r.closes[idx-r.shortROC]-1)
	r.roc = append(r.roc, value)
	if len(r.roc) > r.wmaPeriod {
		r.roc = r.roc[1:]
	}
	if len(r.roc) < r.wmaPeriod {
		r.result = ScalarResult{Timestamp: t}
		return
	}
	sum := 0.0
	for i, x := range r.roc {
		sum += float64(i+1) * x
	}
	current := sum / (float64(r.wmaPeriod) * float64(r.wmaPeriod+1) / 2)
	previous := current
	if r.hasResult {
		previous = r.result.Current
	}
	r.result = ScalarResult{IsReady: true, Current: current, Previous: previous, Timestamp: t}
	r.hasResult = true
}
