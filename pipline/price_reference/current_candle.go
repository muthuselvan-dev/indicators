package price_reference

import (
	"fmt"
	"indicators/core"
	"math"
	"strings"
	"time"
)

// describe: Current Candle
//
// QuantMan:
// type: currentCandle
// name: current
// config: {candleInterval: "1 minutes"}
//
// Original Reference:
// Current = latest accepted candle.
// Previous = preceding accepted candle.
//
// No additional OHLCV snapshot structs.
// Reuses core.MarketCandle.
//
// Warmup:
// First valid candle is immediately ready.
//
// First ready result:
// Previous = Current.
//

type CurrentCandleConfig struct {
	CandleInterval string
}

type CurrentCandle struct {
	id        string
	cfg       CurrentCandleConfig
	lastTime  time.Time
	hasResult bool
	result    CurrentCandleResult
}

func NewCurrentCandle(id string, cfg map[string]any) (*CurrentCandle, error) {
	if id == "" {
		return nil, fmt.Errorf("CurrentCandle: name cannot be empty")
	}

	interval := "1 minutes"

	if value, exists := cfg["candleInterval"]; exists {
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("CurrentCandle: candleInterval must be a string")
		}

		s = strings.TrimSpace(s)
		if s == "" {
			return nil, fmt.Errorf("CurrentCandle: candleInterval cannot be empty")
		}

		interval = s
	}

	return &CurrentCandle{
		id: id,
		cfg: CurrentCandleConfig{
			CandleInterval: interval,
		},
	}, nil
}

func (r *CurrentCandle) Update(c core.MarketCandle) any {
	r.ingest(c)
	return r.result
}

func (r *CurrentCandle) Result() any {
	return r.result
}

func (r *CurrentCandle) ingest(c core.MarketCandle) {
	if !validCurrentCandle(c, r.lastTime) {
		return
	}

	t := c.GetTime()

	previous := c

	if r.hasResult {
		previous = r.result.Current
	}

	r.result = CurrentCandleResult{
		IsReady:   true,
		Current:   c,
		Previous:  previous,
		Timestamp: t,
	}

	r.lastTime = t
	r.hasResult = true
}

func validCurrentCandle(c core.MarketCandle, lastTime time.Time) bool {
	if c == nil {
		return false
	}

	t := c.GetTime()

	if t.IsZero() {
		return false
	}

	if !lastTime.IsZero() && !t.After(lastTime) {
		return false
	}

	o := c.GetOpen()
	h := c.GetHigh()
	l := c.GetLow()
	cl := c.GetClose()
	volume := c.GetVolume()
	oi := c.GetOpenInterest()

	for _, value := range []float64{o, h, l, cl} {
		if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
			return false
		}
	}

	for _, value := range []float64{volume, oi} {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			return false
		}

	}

	if h < l || h < math.Max(o, cl) || l > math.Min(o, cl) {
		return false
	}

	return true
}
