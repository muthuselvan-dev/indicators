package candlestick

import (
	"fmt"
	"indicators/core"
	"time"
)

// describe: ThreeWhiteSoldiers - Three consecutive ascending bullish real bodies.
//
// pipeConfigs: [
//
//	{type: 'threeWhiteSoldiers', name: 'threeWhiteSoldiers', config: {tolerance: 0.1, candleInterval: '1 minutes'}}
//
// ]
//
// Available result:
//   - Current (pattern detected), Previous (preceding ready detection), IsReady, Timestamp
//
// Entry Condition:
//
//	Pattern Is True: Current == true
//	Pattern Becomes True: !Previous && Current
//
// Exit Condition:
//
//	Pattern Is False: Current == false
//	Pattern Becomes False: Previous && !Current
//
// Original Pattern Definition:
//
//	Three consecutive ascending bullish real bodies. Thresholds (10% doji tolerance, 30% small-body threshold, etc.) are configurable or documented implementation conventions; they are not a unique universal formula.
//	Detection needs 3 candle(s). This is a rule-based pattern detector, not a mathematically unique indicator.
//	Heikin-Ashi candles are calculated recursively when applicable.
//
// Reference: https://chartschool.stockcharts.com/table-of-contents/chart-analysis/candlestick-charts/candlestick-pattern-dictionary
// QuantMan's pattern tolerance, trend context and parity have not been verified.
type ThreeWhiteSoldiersConfig struct {
	Tolerance      float64
	CandleInterval string
}
type ThreeWhiteSoldiers struct {
	id              string
	tolerance       float64
	candleInterval  string
	history         []patternCandle
	previousHAOpen  float64
	previousHAClose float64
	hasHA           bool
	lastTimestamp   time.Time
	result          PatternResult
	hasResult       bool
}

func NewThreeWhiteSoldiers(id string, cfg ThreeWhiteSoldiersConfig) (*ThreeWhiteSoldiers, error) {
	if id == "" {
		return nil, fmt.Errorf("ThreeWhiteSoldiers name cannot be empty")
	}
	if cfg.Tolerance < 0 || cfg.Tolerance >= 1 {
		return nil, fmt.Errorf("tolerance must be in [0,1)")
	}
	return &ThreeWhiteSoldiers{id: id, tolerance: cfg.Tolerance, candleInterval: cfg.CandleInterval}, nil
}
func (r *ThreeWhiteSoldiers) Update(candle core.MarketCandle) any { r.ingest(candle); return r.result }
func (r *ThreeWhiteSoldiers) Result() any                         { return r.result }
func (r *ThreeWhiteSoldiers) ingest(candle core.MarketCandle) {
	if !valid(candle, r.lastTimestamp) {
		r.result = PatternResult{Timestamp: func() time.Time {
			if candle == nil {
				return time.Time{}
			}
			return candle.GetTime()
		}()}
		return
	}
	r.lastTimestamp = candle.GetTime()
	c := candleFrom(candle)
	r.history = append(r.history, c)
	if len(r.history) > 3 {
		r.history = r.history[1:]
	}
	if len(r.history) < 3 {
		r.result = PatternResult{Timestamp: r.lastTimestamp}
		return
	}
	tol := r.tolerance
	if tol == 0 {
		tol = 0.1
	}
	a, b, c := r.history[0], r.history[1], r.history[2]
	detected := a.bull() && b.bull() && c.bull() && a.close < b.close && b.close < c.close && b.open > a.open && b.open < a.close && c.open > b.open && c.open < b.close
	previous := detected
	if r.hasResult {
		previous = r.result.Current
	}
	r.result = PatternResult{IsReady: true, Current: detected, Previous: previous, Timestamp: r.lastTimestamp}
	r.hasResult = true
}
