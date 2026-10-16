package price_volume

import (
	"fmt"
	"indicators/core"
	"math"
	"strings"
	"time"
)

// describe: Opening Range Breakout (ORB)
//
// Original Formula:
//   OpeningHigh = Highest High during opening range
//   OpeningLow  = Lowest Low during opening range
//
//   Bullish Breakout: Close > OpeningHigh
//   Bearish Breakout: Close < OpeningLow
//
// Current = Close - OpeningHigh
// Previous = Previous ready Current
//
// QuantMan:
//   type: "openingRangeBreakout"
//   name: "orb"
//   config:
//     candleInterval: "1 minutes"
//
// Additional local configuration:
//   openingRangeMinutes: required
//   sessionStart:        default "09:15:00"
//
// NOTE:
// QuantMan opening-range duration is not verified.
// This implementation requires an explicit duration.
//
// Assumptions:
//   Candle timestamps represent candle opening times.
//   Session dates use candle timestamp timezone.
//   Candles are supplied in chronological order.
//   The opening range is frozen after its duration.
//   A complete opening window is required.

type OpeningRangeBreakoutConfig struct {
	CandleInterval      string
	OpeningRangeMinutes int
	SessionStart        string
}

type OpeningRangeBreakout struct {
	id  string
	cfg OpeningRangeBreakoutConfig

	lastTime time.Time
	day      string

	rangeStart time.Time
	rangeEnd   time.Time

	high float64
	low  float64

	hasRange  bool
	hasResult bool

	result OpeningRangeBreakoutResult
}

func NewOpeningRangeBreakout(id string, cfg map[string]any) (*OpeningRangeBreakout, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("openingRangeBreakout: id required")
	}

	config := OpeningRangeBreakoutConfig{
		CandleInterval: "1 minutes",
		SessionStart:   "09:15:00",
	}

	if value, ok := cfg["candleInterval"]; ok {
		v, valid := value.(string)
		if !valid || strings.TrimSpace(v) == "" {
			return nil, fmt.Errorf("openingRangeBreakout: invalid candleInterval")
		}
		config.CandleInterval = strings.TrimSpace(v)
	}

	if value, ok := cfg["openingRangeMinutes"]; ok {
		n, valid := orbPositiveInteger(value)
		if !valid {
			return nil, fmt.Errorf("openingRangeBreakout: openingRangeMinutes must be a positive integer")
		}
		config.OpeningRangeMinutes = n
	}

	if config.OpeningRangeMinutes <= 0 {
		return nil, fmt.Errorf("openingRangeBreakout: openingRangeMinutes required; QuantMan default is unknown")
	}

	if value, ok := cfg["sessionStart"]; ok {
		v, valid := value.(string)
		if !valid {
			return nil, fmt.Errorf("openingRangeBreakout: invalid sessionStart")
		}
		config.SessionStart = v
	}

	if _, err := time.Parse("15:04:05", config.SessionStart); err != nil {
		return nil, fmt.Errorf("openingRangeBreakout: sessionStart must use HH:MM:SS")
	}

	if config.CandleInterval != "1 minutes" {
		return nil, fmt.Errorf("openingRangeBreakout: only 1 minutes candles supported")
	}

	return &OpeningRangeBreakout{
		id:  id,
		cfg: config,
	}, nil
}

func (r *OpeningRangeBreakout) Update(candle core.MarketCandle) any {
	r.ingest(candle)
	return r.result
}

func (r *OpeningRangeBreakout) Result() any {
	return r.result
}

func (r *OpeningRangeBreakout) ingest(candle core.MarketCandle) {
	if !validOpeningRangeBreakoutCandle(candle, r.lastTime) {
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

	if t.Before(r.rangeStart) {
		r.result = OpeningRangeBreakoutResult{Timestamp: t}
		return
	}

	// Accumulate only candles inside opening range.
	if t.Before(r.rangeEnd) {
		if !r.hasRange {
			r.high = high
			r.low = low
			r.hasRange = true
		} else {
			r.high = math.Max(r.high, high)
			r.low = math.Min(r.low, low)
		}

		r.result = OpeningRangeBreakoutResult{
			IsReady:   false,
			High:      r.high,
			Low:       r.low,
			Timestamp: t,
		}
		return
	}

	// No valid opening candles: remain not ready.
	if !r.hasRange {
		r.result = OpeningRangeBreakoutResult{Timestamp: t}
		return
	}

	// Opening range is now frozen.
	current := closePrice - r.high

	previous := current
	if r.hasResult {
		previous = r.result.Current
	}

	breakout := 0
	if closePrice > r.high {
		breakout = 1
	} else if closePrice < r.low {
		breakout = -1
	}

	r.result = OpeningRangeBreakoutResult{
		IsReady:   true,
		Current:   current,
		Previous:  previous,
		High:      r.high,
		Low:       r.low,
		Breakout:  breakout,
		Timestamp: t,
	}

	r.hasResult = true
}

func (r *OpeningRangeBreakout) resetSession(t time.Time, day string) {
	r.day = day
	r.hasRange = false
	r.hasResult = false
	r.high = 0
	r.low = 0
	r.result = OpeningRangeBreakoutResult{}

	start, _ := time.Parse("15:04:05", r.cfg.SessionStart)

	r.rangeStart = time.Date(
		t.Year(), t.Month(), t.Day(),
		start.Hour(), start.Minute(), start.Second(),
		0, t.Location(),
	)

	r.rangeEnd = r.rangeStart.Add(
		time.Duration(r.cfg.OpeningRangeMinutes) * time.Minute,
	)
}

func orbPositiveInteger(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, v > 0
	case int64:
		if v <= 0 || v > int64(^uint(0)>>1) {
			return 0, false
		}
		return int(v), true
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) ||
			v <= 0 || v != math.Trunc(v) ||
			v > float64(int(^uint(0)>>1)) {
			return 0, false
		}
		return int(v), true
	default:
		return 0, false
	}
}

func validOpeningRangeBreakoutCandle(candle core.MarketCandle, lastTime time.Time) bool {
	if candle == nil {
		return false
	}

	t := candle.GetTime()
	if t.IsZero() || (!lastTime.IsZero() && !t.After(lastTime)) {
		return false
	}

	o := candle.GetOpen()
	h := candle.GetHigh()
	l := candle.GetLow()
	c := candle.GetClose()

	for _, value := range []float64{o, h, l, c} {
		if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
			return false
		}
	}

	if h < l || h < math.Max(o, c) || l > math.Min(o, c) {
		return false
	}

	return true
}
