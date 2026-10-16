package price_reference

import (
	"fmt"
	"indicators/core"
	"indicators/datasource"

	"math"
	"strings"
	"time"
)

// Previous Day Candle / Yesterday Candle
//
// Current = previous completed trading day's OHLCV/OI.
// Previous = completed day before that, when available.
//
// Config: {} (QuantMan yesterdayCandle)
//
// No period or smoothing formula.
//
// Assumption: trading session follows candle's calendar date.
// Exact QuantMan session semantics require verification.

type PreviousDayCandle struct {
	id string

	lastTime   time.Time
	sessionDay string

	today    datasource.Candle
	hasToday bool

	yesterday    datasource.Candle
	hasYesterday bool

	dayBefore    datasource.Candle
	hasDayBefore bool

	result PreviousDayCandleResult
}

func NewPreviousDayCandle(id string, cfg map[string]any) (*PreviousDayCandle, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("PreviousDayCandle: empty name")
	}

	// No indicator-specific config is required by yesterdayCandle.
	return &PreviousDayCandle{id: id}, nil
}

func (r *PreviousDayCandle) Update(c core.MarketCandle) any {
	r.ingest(c)
	return r.result
}

func (r *PreviousDayCandle) Result() any {
	return r.result
}

func (r *PreviousDayCandle) ingest(c core.MarketCandle) {
	if !validPreviousDayCandle(c, r.lastTime) {
		return
	}

	t := c.GetTime()
	day := t.Format("2006-01-02")

	if !r.hasToday {
		r.today = datasource.Candle{
			Timestamp:    t,
			Open:         c.GetOpen(),
			High:         c.GetHigh(),
			Low:          c.GetLow(),
			Close:        c.GetClose(),
			Volume:       c.GetVolume(),
			OpenInterest: c.GetOpenInterest(),
		}
		r.hasToday = true
		r.sessionDay = day
		r.lastTime = t
		r.result = PreviousDayCandleResult{Timestamp: t}
		return
	}

	if day != r.sessionDay {
		if r.hasYesterday {
			r.dayBefore = r.yesterday
			r.hasDayBefore = true
		}

		r.yesterday = r.today
		r.hasYesterday = true

		r.today = datasource.Candle{
			Timestamp:    t,
			Open:         c.GetOpen(),
			High:         c.GetHigh(),
			Low:          c.GetLow(),
			Close:        c.GetClose(),
			Volume:       c.GetVolume(),
			OpenInterest: c.GetOpenInterest(),
		}

		r.sessionDay = day
	} else {
		if c.GetHigh() > r.today.High {
			r.today.High = c.GetHigh()
		}
		if c.GetLow() < r.today.Low {
			r.today.Low = c.GetLow()
		}

		r.today.Close = c.GetClose()
		r.today.Volume += c.GetVolume()
		r.today.OpenInterest = c.GetOpenInterest()
	}

	r.lastTime = t

	if !r.hasYesterday {
		r.result = PreviousDayCandleResult{Timestamp: t}
		return
	}

	var previous core.MarketCandle = r.yesterday

	if r.hasDayBefore {
		previous = r.dayBefore
	}

	r.result = PreviousDayCandleResult{
		IsReady:   true,
		Current:   r.yesterday,
		Previous:  previous,
		Timestamp: t,
	}
}

func validPreviousDayCandle(c core.MarketCandle, lastTime time.Time) bool {
	if c == nil {
		return false
	}

	t := c.GetTime()
	if t.IsZero() || (!lastTime.IsZero() && !t.After(lastTime)) {
		return false
	}

	o, h, l, cl := c.GetOpen(), c.GetHigh(), c.GetLow(), c.GetClose()
	v, oi := c.GetVolume(), c.GetOpenInterest()

	for _, x := range []float64{o, h, l, cl} {
		if math.IsNaN(x) || math.IsInf(x, 0) || x <= 0 {
			return false
		}
	}

	for _, x := range []float64{v, oi} {
		if math.IsNaN(x) || math.IsInf(x, 0) || x < 0 {
			return false
		}
	}

	return h >= l && h >= math.Max(o, cl) && l <= math.Min(o, cl)
}
