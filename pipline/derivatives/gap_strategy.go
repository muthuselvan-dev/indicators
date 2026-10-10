package derivatives

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: Gap Strategy
//
// pipeConfigs: [
//   {type: 'gapStrategy', name: 'gapStrategy', config: {period: 14, candleInterval: '1 minutes'}},
// ]
//
// Available result:
//   - Current, Previous, IsReady, Timestamp
//
// Entry Condition:
//   Is Above threshold: Current > threshold
//   Crosses Above threshold: Previous <= threshold && Current > threshold
//
// Exit Condition:
//   Is Below threshold: Current < threshold
//   Crosses Below threshold: Previous >= threshold && Current < threshold
//
// Original Formula / Explicit Interpretation:
//   Gap = TodayFirstOpen - PreviousDayClose; Current is gap size.
//
// Source: Standard OHLC arithmetic; QuantMan-specific semantics not verified.
// Example config values are NOT claimed QuantMan defaults.
// Warm-up: depends on period and prior-session availability.

type GapStrategyConfig struct {
	Period         int
	Multiplier     float64
	MaxBodyRatio   float64
	CandleInterval string
}
type GapStrategy struct {
	id               string
	cfg              GapStrategyConfig
	candles          []gapstrategySnapshotInternal
	lastTime         time.Time
	hasResult        bool
	result           ScalarResult
	sessionDay       string
	sessionOpen      float64
	sessionHigh      float64
	sessionLow       float64
	previousDayClose float64
	lastClose        float64
	hasPreviousDay   bool
}
type GapStrategySnapshot struct {
	Open, High, Low, Close float64
	Timestamp              time.Time
}
type gapstrategySnapshotInternal struct {
	open, high, low, close float64
	timestamp              time.Time
}

func NewGapStrategy(id string, cfg GapStrategyConfig) (*GapStrategy, error) {
	if id == "" || cfg.Period <= 0 {
		return nil, fmt.Errorf("Gap Strategy: name and positive period required")
	}
	if cfg.Multiplier < 0 || cfg.MaxBodyRatio < 0 {
		return nil, fmt.Errorf("Gap Strategy: invalid multiplier/ratio")
	}
	return &GapStrategy{id: id, cfg: cfg}, nil
}
func (r *GapStrategy) Update(c core.MarketCandle) any { r.ingest(c); return r.result }
func (r *GapStrategy) Result() any                    { return r.result }
func (r *GapStrategy) ingest(c core.MarketCandle) {
	if c == nil {
		return
	}
	t := c.GetTime()
	o, h, l, cl := c.GetOpen(), c.GetHigh(), c.GetLow(), c.GetClose()
	if t.IsZero() || (!r.lastTime.IsZero() && !t.After(r.lastTime)) || o <= 0 || h < l || h < math.Max(o, cl) || l > math.Min(o, cl) || math.IsNaN(cl) || math.IsInf(cl, 0) {
		r.result = ScalarResult{Timestamp: t}
		return
	}
	r.lastTime = t
	day := t.Format("2006-01-02")
	newDay := day != r.sessionDay
	if newDay {
		if r.sessionDay != "" {
			r.previousDayClose = r.lastClose
			r.hasPreviousDay = true
		}
		r.sessionDay = day
		r.sessionOpen = o
		r.sessionHigh = h
		r.sessionLow = l
	} else {
		if h > r.sessionHigh {
			r.sessionHigh = h
		}
		if l < r.sessionLow {
			r.sessionLow = l
		}
	}
	r.lastClose = cl
	s := gapstrategySnapshotInternal{open: o, high: h, low: l, close: cl, timestamp: t}
	previous := gapstrategySnapshotInternal{}
	hasPrevious := len(r.candles) > 0
	if hasPrevious {
		previous = r.candles[len(r.candles)-1]
	}
	r.candles = append(r.candles, s)
	n := r.cfg.Period
	if len(r.candles) > n+2 {
		r.candles = r.candles[len(r.candles)-(n+2):]
	}
	current := 0.0
	ready := false
	switch "gap" {
	case "previous":
		if hasPrevious {
			current = previous.close
			ready = true
		}
	case "indecision", "indecision_value":
		if hasPrevious && previous.high > previous.low {
			ratio := math.Abs(previous.close-previous.open) / (previous.high - previous.low)
			if "gap" == "indecision_value" {
				current = ratio
			} else if ratio <= r.cfg.MaxBodyRatio {
				current = 1
			}
			ready = true
		}
	case "at":
		if len(r.candles) > n {
			current = r.candles[len(r.candles)-1-n].close
			ready = true
		}
	case "lastn":
		if len(r.candles) >= n {
			for _, x := range r.candles[len(r.candles)-n:] {
				current += x.close
			}
			current /= float64(n)
			ready = true
		}
	case "today":
		current = cl
		ready = true
	case "prevday":
		if r.hasPreviousDay {
			current = r.previousDayClose
			ready = true
		}
	case "gap":
		if r.hasPreviousDay {
			current = r.sessionOpen - r.previousDayClose
			ready = true
		}
	case "orb":
		if len(r.candles) >= n {
			high := r.candles[len(r.candles)-n].high
			for _, x := range r.candles[len(r.candles)-n:] {
				if x.high > high {
					high = x.high
				}
			}
			current = cl - high
			ready = true
		}
	case "range":
		if len(r.candles) > n {
			high := r.candles[len(r.candles)-1-n].high
			for _, x := range r.candles[len(r.candles)-1-n : len(r.candles)-1] {
				if x.high > high {
					high = x.high
				}
			}
			current = cl - high
			ready = true
		}
	case "mrc":
		if len(r.candles) >= n {
			for _, x := range r.candles[len(r.candles)-n:] {
				current += x.close
			}
			current /= float64(n)
			ready = true
		}
	case "atrma":
		if len(r.candles) >= n+1 {
			for i := len(r.candles) - n; i < len(r.candles); i++ {
				x, p := r.candles[i], r.candles[i-1]
				tr := math.Max(x.high-x.low, math.Max(math.Abs(x.high-p.close), math.Abs(x.low-p.close)))
				current += tr
			}
			current /= float64(n)
			ready = true
		}
	case "rsima":
		// RSI-of-RSI smoothing requires a separate RSI lookback; fail closed until configured.
		ready = false
	}
	if !ready {
		r.result = ScalarResult{Timestamp: t}
		return
	}
	prev := current
	if r.hasResult {
		prev = r.result.Current
	}
	r.result = ScalarResult{IsReady: true, Current: current, Previous: prev, Timestamp: t}
	r.hasResult = true
}

var _ core.Indicator = (*GapStrategy)(nil)
