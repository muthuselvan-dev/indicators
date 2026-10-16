package price_volume

import (
	"fmt"
	"indicators/core"
	"math"
	"strings"
	"time"
)

// describe: Time Window Range Breakout
//
// QuantMan Config:
//   type: "candleAt"
//   name: "RangeBreakout"
//   config:
//     candleInterval: "1 minutes"
//     startTime: "09:30"
//     endTime: "10:30"
//
// Standard Range Breakout Formula:
//   RangeHigh = MAX(High) inside configured time window
//   RangeLow  = MIN(Low) inside configured time window
//
//   Current  = Close - RangeHigh
//   Breakout =  1 when Close > RangeHigh
//             -1 when Close < RangeLow
//              0 otherwise
//
// Range is frozen after the configured end time.
// Daily reset uses the candle timestamp's timezone.
//
// NOTE:
// QuantMan candleAt semantics are not independently verified.
// This is a time-window breakout implementation, not
// a confirmed reproduction of QuantMan's candleAt engine.
//
// Timestamp assumption:
// Candle timestamps represent candle opening times.
// The configured range includes startTime and excludes endTime.

type RangeBreakoutConfig struct {
	CandleInterval string
	StartTime      string
	EndTime        string
}

type RangeBreakout struct {
	id  string
	cfg RangeBreakoutConfig

	lastTime time.Time
	day      string

	rangeStart time.Time
	rangeEnd   time.Time

	rangeHigh float64
	rangeLow  float64
	hasRange  bool

	hasResult bool
	result    RangeBreakoutResult
}

func NewRangeBreakout(id string, cfg map[string]any) (*RangeBreakout, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("rangeBreakout: indicator id required")
	}

	config := RangeBreakoutConfig{
		CandleInterval: "1 minutes",
		StartTime:      "09:30",
		EndTime:        "10:30",
	}

	if value, exists := cfg["candleInterval"]; exists {
		v, ok := value.(string)
		if !ok || strings.TrimSpace(v) == "" {
			return nil, fmt.Errorf("rangeBreakout: invalid candleInterval")
		}
		config.CandleInterval = strings.TrimSpace(v)
	}

	if value, exists := cfg["startTime"]; exists {
		v, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("rangeBreakout: invalid startTime")
		}
		config.StartTime = v
	}

	if value, exists := cfg["endTime"]; exists {
		v, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("rangeBreakout: invalid endTime")
		}
		config.EndTime = v
	}

	start, err := time.Parse("15:04", config.StartTime)
	if err != nil {
		return nil, fmt.Errorf("rangeBreakout: startTime must be HH:MM")
	}

	end, err := time.Parse("15:04", config.EndTime)
	if err != nil {
		return nil, fmt.Errorf("rangeBreakout: endTime must be HH:MM")
	}

	startMinute := start.Hour()*60 + start.Minute()
	endMinute := end.Hour()*60 + end.Minute()

	if endMinute <= startMinute {
		return nil, fmt.Errorf("rangeBreakout: endTime must be after startTime")
	}

	if config.CandleInterval != "1 minutes" {
		return nil, fmt.Errorf("rangeBreakout: only 1 minutes candles supported")
	}

	return &RangeBreakout{
		id:  id,
		cfg: config,
	}, nil
}

func (r *RangeBreakout) Update(candle core.MarketCandle) any {
	r.ingest(candle)
	return r.result
}

func (r *RangeBreakout) Result() any {
	return r.result
}

func (r *RangeBreakout) ingest(candle core.MarketCandle) {
	if !validRangeBreakoutCandle(candle, r.lastTime) {
		return
	}

	t := candle.GetTime()
	high := candle.GetHigh()
	low := candle.GetLow()
	closePrice := candle.GetClose()

	r.lastTime = t

	day := t.Format("2006-01-02")

	if day != r.day {
		r.resetSession(t, day)
	}

	// Before configured range: not ready.
	if t.Before(r.rangeStart) {
		r.result = RangeBreakoutResult{
			Timestamp: t,
		}
		return
	}

	// Build range using candles within [start, end).
	if t.Before(r.rangeEnd) {
		if !r.hasRange {
			r.rangeHigh = high
			r.rangeLow = low
			r.hasRange = true
		} else {
			r.rangeHigh = math.Max(r.rangeHigh, high)
			r.rangeLow = math.Min(r.rangeLow, low)
		}

		r.result = RangeBreakoutResult{
			IsReady:   false,
			High:      r.rangeHigh,
			Low:       r.rangeLow,
			Timestamp: t,
		}
		return
	}

	// Cannot calculate without a completed range.
	if !r.hasRange {
		r.result = RangeBreakoutResult{
			Timestamp: t,
		}
		return
	}

	// RangeHigh and RangeLow are frozen after endTime.
	current := closePrice - r.rangeHigh

	previous := current
	if r.hasResult {
		previous = r.result.Current
	}

	breakout := 0

	if closePrice > r.rangeHigh {
		breakout = 1
	} else if closePrice < r.rangeLow {
		breakout = -1
	}

	r.result = RangeBreakoutResult{
		IsReady:   true,
		Current:   current,
		Previous:  previous,
		High:      r.rangeHigh,
		Low:       r.rangeLow,
		Breakout:  breakout,
		Timestamp: t,
	}

	r.hasResult = true
}

func (r *RangeBreakout) resetSession(t time.Time, day string) {
	r.day = day

	r.hasRange = false
	r.hasResult = false
	r.rangeHigh = 0
	r.rangeLow = 0
	r.result = RangeBreakoutResult{}

	start, _ := time.Parse("15:04", r.cfg.StartTime)
	end, _ := time.Parse("15:04", r.cfg.EndTime)

	r.rangeStart = time.Date(
		t.Year(), t.Month(), t.Day(),
		start.Hour(), start.Minute(), 0, 0, t.Location(),
	)

	r.rangeEnd = time.Date(
		t.Year(), t.Month(), t.Day(),
		end.Hour(), end.Minute(), 0, 0, t.Location(),
	)
}

func validRangeBreakoutCandle(candle core.MarketCandle, lastTime time.Time) bool {
	if candle == nil {
		return false
	}

	t := candle.GetTime()

	if t.IsZero() {
		return false
	}

	if !lastTime.IsZero() && !t.After(lastTime) {
		return false
	}

	o := candle.GetOpen()
	h := candle.GetHigh()
	l := candle.GetLow()
	cl := candle.GetClose()

	for _, value := range []float64{o, h, l, cl} {
		if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
			return false
		}
	}

	if h < l || h < math.Max(o, cl) || l > math.Min(o, cl) {
		return false
	}

	return true
}
