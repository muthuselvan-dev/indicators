package price_reference

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: Previous Candle
//
// QuantMan pipeConfigs:
// {
//   type: "previousCandle",
//   name: "previous",
//   config: {
//     candleInterval: "1 minutes"
//   }
// }
//
// Current  = Immediately previous candle.
// Previous = Reference from previous ready update.
//
// Warmup: 2 valid candles.
// QuantMan exact parity not yet verified.

type PreviousCandle struct {
	id             string
	candleInterval string

	lastCandle    core.MarketCandle
	lastTimestamp time.Time
	hasResult     bool

	result PreviousCandleResult
}

func NewPreviousCandle(id string, cfg map[string]any) (*PreviousCandle, error) {
	if id == "" {
		return nil, fmt.Errorf("PreviousCandle name cannot be empty")
	}

	interval, _ := cfg["candleInterval"].(string)

	return &PreviousCandle{
		id:             id,
		candleInterval: interval,
	}, nil
}

func (r *PreviousCandle) Update(candle core.MarketCandle) any {
	r.ingest(candle)
	return r.result
}

func (r *PreviousCandle) Result() any {
	return r.result
}

func (r *PreviousCandle) ingest(candle core.MarketCandle) {
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
		!validPreviousCandleOHLC(o, h, l, c) ||
		!validPreviousCandleValue(v) ||
		!validPreviousCandleValue(oi) {
		return
	}

	r.lastTimestamp = t

	// First candle: warmup.
	if r.lastCandle == nil {
		r.lastCandle = candle

		r.result = PreviousCandleResult{
			IsReady:   false,
			Timestamp: t,
		}
		return
	}

	// Immediately preceding candle.
	selected := r.lastCandle

	previous := selected
	if r.hasResult {
		previous = r.result.Current
	}

	r.result = PreviousCandleResult{
		IsReady:   true,
		Current:   selected,
		Previous:  previous,
		Timestamp: t,
	}

	r.lastCandle = candle
	r.hasResult = true
}

func validPreviousCandleOHLC(o, h, l, c float64) bool {
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

func validPreviousCandleValue(value float64) bool {
	return !math.IsNaN(value) &&
		!math.IsInf(value, 0) &&
		value >= 0
}
