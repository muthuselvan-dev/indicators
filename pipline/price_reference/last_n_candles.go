package price_reference

import (
	"fmt"
	"indicators/core"
	"math"
	"strings"
	"time"
)

// describe: Last N Candles
//
// Config:
// period: 7
// candleInterval: "1 minutes"
//
// Current = latest N accepted candles.
// Previous = preceding N-candle window.
//
// No SMA calculation.
// No additional OHLCV snapshot structs.
//
// Note: Rolling-window behavior is a local interpretation.
// QuantMan exact parity is not yet verified.

type LastNCandlesConfig struct {
	Period         int
	CandleInterval string
}

type LastNCandles struct {
	id       string
	cfg      LastNCandlesConfig
	candles  []core.MarketCandle
	lastTime time.Time
	result   LastNCandlesResult
}

func NewLastNCandles(id string, cfg map[string]any) (*LastNCandles, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("LastNCandles: empty name")
	}

	config := LastNCandlesConfig{
		CandleInterval: "1 minutes",
	}

	raw, exists := cfg["period"]
	if !exists {
		return nil, fmt.Errorf("LastNCandles: period required")
	}

	switch v := raw.(type) {
	case int:
		config.Period = v
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) ||
			math.Trunc(v) != v ||
			v > float64(int(^uint(0)>>1)) {
			return nil, fmt.Errorf("LastNCandles: invalid period")
		}
		config.Period = int(v)
	default:
		return nil, fmt.Errorf("LastNCandles: invalid period type")
	}

	if config.Period <= 0 {
		return nil, fmt.Errorf("LastNCandles: period must be positive")
	}

	if value, exists := cfg["candleInterval"]; exists {
		s, ok := value.(string)
		if !ok || strings.TrimSpace(s) == "" {
			return nil, fmt.Errorf("LastNCandles: invalid candleInterval")
		}
		config.CandleInterval = strings.TrimSpace(s)
	}

	return &LastNCandles{
		id:      id,
		cfg:     config,
		candles: make([]core.MarketCandle, 0, config.Period+1),
	}, nil
}

func (r *LastNCandles) Update(c core.MarketCandle) any {
	r.ingest(c)
	return r.result
}

func (r *LastNCandles) Result() any {
	return r.result
}

func (r *LastNCandles) ingest(c core.MarketCandle) {
	if !validLastNCandle(c, r.lastTime) {
		return
	}

	t := c.GetTime()
	n := r.cfg.Period

	r.lastTime = t
	r.candles = append(r.candles, c)

	if len(r.candles) > n+1 {
		copy(r.candles, r.candles[1:])
		r.candles = r.candles[:n+1]
	}

	if len(r.candles) < n {
		r.result = LastNCandlesResult{
			Timestamp: t,
		}
		return
	}

	current := make([]core.MarketCandle, n)
	copy(current, r.candles[len(r.candles)-n:])

	var previous []core.MarketCandle

	if len(r.candles) >= n+1 {
		previous = make([]core.MarketCandle, n)
		copy(previous, r.candles[len(r.candles)-n-1:len(r.candles)-1])
	} else {
		previous = current
	}

	r.result = LastNCandlesResult{
		IsReady:   true,
		Current:   current,
		Previous:  previous,
		Timestamp: t,
	}
}

func validLastNCandle(c core.MarketCandle, lastTime time.Time) bool {
	if c == nil {
		return false
	}

	t := c.GetTime()
	if t.IsZero() || (!lastTime.IsZero() && !t.After(lastTime)) {
		return false
	}

	o := c.GetOpen()
	h := c.GetHigh()
	l := c.GetLow()
	cl := c.GetClose()
	v := c.GetVolume()
	oi := c.GetOpenInterest()

	for _, x := range []float64{o, h, l, cl} {
		if math.IsNaN(x) || math.IsInf(x, 0) || x <= 0 {
			return false
		}
	}

	for _, x := range []float64{v, oi} {
		if math.IsNaN(x) || math.IsInf(x, 0) || x <= 0 {
			if x < 0 || math.IsNaN(x) || math.IsInf(x, 0) {
				return false
			}
		}
	}

	if h < l || h < math.Max(o, cl) || l > math.Min(o, cl) {
		return false
	}

	return true
}
