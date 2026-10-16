package price_reference

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: Previous Nth Candle
//
// pipeConfigs:
// type: previousNthCandle
// name: previousNthCandle
// config:
//   period: 10
//   candleInterval: "1 minutes"
//
// Current = candle N positions before current.
// Requires N+1 valid candles.
//
// QuantMan exact reference parity not verified.

type PreviousNthCandle struct {
	id string

	period         int
	candleInterval string

	history []PreviousNthCandleSnapshot

	lastTimestamp time.Time
	hasResult     bool

	result PreviousNthCandleResult
}

func NewPreviousNthCandle(id string, cfg map[string]any) (*PreviousNthCandle, error) {
	if id == "" {
		return nil, fmt.Errorf("PreviousNthCandle name cannot be empty")
	}

	period, ok := cfg["period"].(int)
	if !ok || period <= 0 {
		return nil, fmt.Errorf("PreviousNthCandle requires positive period")
	}

	candleInterval, _ := cfg["candleInterval"].(string)

	return &PreviousNthCandle{
		id:             id,
		period:         period,
		candleInterval: candleInterval,
	}, nil
}

func (r *PreviousNthCandle) Update(candle core.MarketCandle) any {
	r.ingest(candle)
	return r.result
}

func (r *PreviousNthCandle) Result() any {
	return r.result
}

func (r *PreviousNthCandle) ingest(candle core.MarketCandle) {
	if candle == nil {
		return
	}

	t := candle.GetTime()
	o := candle.GetOpen()
	h := candle.GetHigh()
	l := candle.GetLow()
	c := candle.GetClose()
	v := candle.GetVolume()
	oi := candle.GetOpenInterest()

	if t.IsZero() ||
		(!r.lastTimestamp.IsZero() && !t.After(r.lastTimestamp)) ||
		!validNthOHLC(o, h, l, c) ||
		!validNthNonNegative(v) ||
		!validNthNonNegative(oi) {
		return
	}

	snapshot := PreviousNthCandleSnapshot{
		Open:         o,
		High:         h,
		Low:          l,
		Close:        c,
		Volume:       v,
		OpenInterest: oi,
		Timestamp:    t,
	}

	r.lastTimestamp = t
	r.history = append(r.history, snapshot)

	if len(r.history) > r.period+1 {
		r.history = r.history[1:]
	}

	if len(r.history) <= r.period {
		r.result = PreviousNthCandleResult{
			IsReady:   false,
			Timestamp: t,
		}
		return
	}

	selected := r.history[len(r.history)-1-r.period]

	previous := selected
	if r.hasResult {
		previous = r.result.Current
	}

	r.result = PreviousNthCandleResult{
		IsReady:   true,
		Current:   selected,
		Previous:  previous,
		Timestamp: t,
	}

	r.hasResult = true
}

func validNthOHLC(o, h, l, c float64) bool {
	for _, value := range []float64{o, h, l, c} {
		if math.IsNaN(value) ||
			math.IsInf(value, 0) ||
			value <= 0 {
			return false
		}
	}

	return h >= math.Max(o, c) &&
		l <= math.Min(o, c)
}

func validNthNonNegative(value float64) bool {
	return !math.IsNaN(value) &&
		!math.IsInf(value, 0) &&
		value >= 0
}
