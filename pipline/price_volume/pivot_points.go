package price_volume

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: PivotPoints - previous completed calendar-day OHLC reference levels.
//
// pipeConfigs: [{type:'pivotPoints',name:'pivotPoints',config:{candleInterval:'1 minutes'}}]
//
// Available result:
//   - Pivot,Resistance1,Support1,Resistance2,Support2 float64, IsReady, Timestamp
//
// Entry Condition:
//
//	Price Is Above Pivot: CurrentClose > Pivot
//	Price Crosses Above Pivot: PreviousClose <= Pivot && CurrentClose > Pivot
//
// Exit Condition:
//
//	Price Is Below Pivot: CurrentClose < Pivot
//	Price Crosses Below Pivot: PreviousClose >= Pivot && CurrentClose < Pivot
//
// Original Formula:
//
//	P=(PreviousDayHigh+PreviousDayLow+PreviousDayClose)/3; R1=2P-L; S1=2P-H; R2=P+(H-L); S2=P-(H-L)
//	Previous completed day is frozen for the current session.
//	Calendar date uses candle's timezone; exchange-session boundaries need separate validation.
//
// Reference: https://chartschool.stockcharts.com/table-of-contents/technical-indicators-and-overlays/technical-overlays/pivot-points
// QuantMan session handling and parity have not been verified.
type PivotPointsConfig struct{ CandleInterval string }
type PivotPointsResult struct {
	IsReady                                             bool
	Pivot, Resistance1, Support1, Resistance2, Support2 float64
	Timestamp                                           time.Time
}
type PivotPoints struct {
	id               string
	candleInterval   string
	lastTimestamp    time.Time
	day              string
	high, low, close float64
	hasDay           bool
	result           PivotPointsResult
}

func NewPivotPoints(id string, cfg PivotPointsConfig) (*PivotPoints, error) {
	if id == "" {
		return nil, fmt.Errorf("name cannot be empty")
	}
	return &PivotPoints{id: id, candleInterval: cfg.CandleInterval}, nil
}
func (r *PivotPoints) Update(c core.MarketCandle) any { r.ingest(c); return r.result }
func (r *PivotPoints) Result() any                    { return r.result }
func (r *PivotPoints) ingest(c core.MarketCandle) {
	if c == nil || c.GetTime().IsZero() || (!r.lastTimestamp.IsZero() && !c.GetTime().After(r.lastTimestamp)) {
		r.result.IsReady = false
		return
	}
	h, l, cl := c.GetHigh(), c.GetLow(), c.GetClose()
	if math.IsNaN(h) || math.IsNaN(l) || math.IsNaN(cl) || math.IsInf(h, 0) || math.IsInf(l, 0) || math.IsInf(cl, 0) || l <= 0 || h < l || cl < l || cl > h {
		r.result.IsReady = false
		return
	}
	r.lastTimestamp = c.GetTime()
	day := c.GetTime().Format("2006-01-02")
	if !r.hasDay {
		r.hasDay = true
		r.day = day
		r.high = h
		r.low = l
		r.close = cl
		r.result = PivotPointsResult{Timestamp: r.lastTimestamp}
		return
	}
	if day != r.day {
		result := PivotPointsResult{IsReady: true, Timestamp: r.lastTimestamp}
		p := (r.high + r.low + r.close) / 3
		result.Pivot = p
		result.Resistance1 = 2*p - r.low
		result.Support1 = 2*p - r.high
		result.Resistance2 = p + (r.high - r.low)
		result.Support2 = p - (r.high - r.low)
		r.result = result
		r.day = day
		r.high = h
		r.low = l
		r.close = cl
		return
	}
	if h > r.high {
		r.high = h
	}
	if l < r.low {
		r.low = l
	}
	r.close = cl
	r.result.Timestamp = r.lastTimestamp
}
