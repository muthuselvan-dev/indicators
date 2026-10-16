package derivatives

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: Gap Strategy
//
// QuantMan pipeConfig:
// type: gapStrategy
// name: gapStrategy
//
// Standard gap calculation:
//   GapPoints  = TodayOpen - PreviousDayClose
//   GapPercent = GapPoints / PreviousDayClose * 100
//
// Gap types (checked against QuantMan, BANKNIFTY 1 min, Aug 2025):
//
//   FullGapUp      = TodayOpen >  PreviousDayHigh
//   FullGapDown    = TodayOpen <  PreviousDayLow
//   PartialGapUp   = TodayOpen >  PreviousDayClose && TodayOpen <= PreviousDayHigh
//   PartialGapDown = TodayOpen <  PreviousDayClose && TodayOpen >= PreviousDayLow
//
// Full gap up / down are verified against QuantMan trades.
// Partial gap up / down use the usual definition and are NOT verified yet.
//
// GapUp   = GapPoints > 0
// GapDown = GapPoints < 0
//
// Current  = GapPoints
// Previous = Previous ready GapPoints
//
// All gap values stay fixed throughout the trading day.
// The first trading day is only used for warm-up.
//
// IMPORTANT: this file defines GapStrategyResult.
// Delete the old GapStrategyResult type from wherever it was declared.

type GapStrategy struct {
	id string

	sessionDay string
	lastTime   time.Time

	// Running values of the current session.
	sessionHigh float64
	sessionLow  float64
	lastClose   float64
	todayOpen   float64

	// Values of the previous trading day.
	previousDayClose float64
	previousDayHigh  float64
	previousDayLow   float64

	hasPreviousDay bool
	hasResult      bool

	result GapStrategyResult
}

func NewGapStrategy(id string, cfg map[string]any) (*GapStrategy, error) {
	if id == "" {
		return nil, fmt.Errorf("GapStrategy name cannot be empty")
	}

	return &GapStrategy{
		id: id,
	}, nil
}

func (r *GapStrategy) Update(candle core.MarketCandle) any {
	r.ingest(candle)
	return r.result
}

func (r *GapStrategy) Result() any {
	return r.result
}

func (r *GapStrategy) ingest(candle core.MarketCandle) {
	if candle == nil {
		r.result.IsReady = false
		return
	}

	t := candle.GetTime()
	o := candle.GetOpen()
	h := candle.GetHigh()
	l := candle.GetLow()
	c := candle.GetClose()

	if t.IsZero() ||
		(!r.lastTime.IsZero() && !t.After(r.lastTime)) ||
		!validGapOHLC(o, h, l, c) {
		r.result.IsReady = false
		return
	}

	day := t.Format("2006-01-02")

	if day != r.sessionDay {
		// A new session starts: the finished session becomes "previous day".
		if r.sessionDay != "" {
			r.previousDayClose = r.lastClose
			r.previousDayHigh = r.sessionHigh
			r.previousDayLow = r.sessionLow
			r.hasPreviousDay = true
		}

		r.sessionDay = day
		r.todayOpen = o
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

	r.lastClose = c
	r.lastTime = t

	// First trading day is used for warmup.
	if !r.hasPreviousDay {
		r.result = GapStrategyResult{
			IsReady:   false,
			Timestamp: t,
		}
		return
	}

	gapPoints := r.todayOpen - r.previousDayClose
	gapPercent := (gapPoints / r.previousDayClose) * 100

	fullGapUp := r.todayOpen > r.previousDayHigh
	fullGapDown := r.todayOpen < r.previousDayLow

	partialGapUp := r.todayOpen > r.previousDayClose &&
		r.todayOpen <= r.previousDayHigh

	partialGapDown := r.todayOpen < r.previousDayClose &&
		r.todayOpen >= r.previousDayLow

	previous := gapPoints

	if r.hasResult {
		previous = r.result.Current
	}

	r.result = GapStrategyResult{
		IsReady:          true,
		Current:          gapPoints,
		Previous:         previous,
		GapPoints:        gapPoints,
		GapPercent:       gapPercent,
		GapUp:            gapPoints > 0,
		GapDown:          gapPoints < 0,
		FullGapUp:        fullGapUp,
		FullGapDown:      fullGapDown,
		PartialGapUp:     partialGapUp,
		PartialGapDown:   partialGapDown,
		TodayOpen:        r.todayOpen,
		PreviousDayClose: r.previousDayClose,
		PreviousDayHigh:  r.previousDayHigh,
		PreviousDayLow:   r.previousDayLow,
		Timestamp:        t,
	}

	r.hasResult = true
}

func validGapOHLC(o, h, l, c float64) bool {
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
