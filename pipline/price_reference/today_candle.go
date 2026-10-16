package price_reference

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: Today Candle
//
// Maintains running OHLC values for the current
// trading session (based on candle date).
//
// QuantMan exact semantics are not verified.

type TodayCandle struct {
	id string

	sessionDay string
	lastTime   time.Time

	sessionOpen  float64
	sessionHigh  float64
	sessionLow   float64
	sessionClose float64

	hasResult bool
	result    TodayCandleResult
}

func NewTodayCandle(id string, cfg map[string]any) (*TodayCandle, error) {
	if id == "" {
		return nil, fmt.Errorf("Today Candle name cannot be empty")
	}

	return &TodayCandle{
		id: id,
	}, nil
}

func (r *TodayCandle) Update(candle core.MarketCandle) any {
	r.ingest(candle)
	return r.result
}

func (r *TodayCandle) Result() any {
	return r.result
}

func (r *TodayCandle) ingest(candle core.MarketCandle) {
	if candle == nil {
		return
	}

	t := candle.GetTime()
	o := candle.GetOpen()
	h := candle.GetHigh()
	l := candle.GetLow()
	c := candle.GetClose()

	if t.IsZero() ||
		(!r.lastTime.IsZero() && !t.After(r.lastTime)) ||
		!validTodayCandleOHLC(o, h, l, c) {
		r.result.IsReady = false
		return
	}

	day := t.Format("2006-01-02")

	if day != r.sessionDay {
		// First candle of a new session.
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

	r.sessionClose = c
	r.lastTime = t

	previous := r.result.Close

	if !r.hasResult {
		previous = c
	}

	r.result = TodayCandleResult{
		IsReady:   true,
		Current:   c,
		Previous:  previous,
		Open:      r.sessionOpen,
		High:      r.sessionHigh,
		Low:       r.sessionLow,
		Close:     r.sessionClose,
		Timestamp: t,
	}

	r.hasResult = true
}

func validTodayCandleOHLC(o, h, l, c float64) bool {
	values := []float64{o, h, l, c}

	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) ||
			value <= 0 {
			return false
		}
	}

	return h >= math.Max(o, c) &&
		l <= math.Min(o, c)
}
