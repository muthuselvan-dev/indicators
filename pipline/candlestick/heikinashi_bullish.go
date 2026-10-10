package candlestick

import (
	"fmt"
	"indicators/core"
	"time"
)

// describe: HeikinashiBullish - Heikin-Ashi transformed close above transformed open.
//
// pipeConfigs: [
//
//	{type: 'heikinashiBullish', name: 'heikinashiBullish', config: {tolerance: 0.1, candleInterval: '1 minutes'}}
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
//	Heikin-Ashi transformed close above transformed open. Thresholds (10% doji tolerance, 30% small-body threshold, etc.) are configurable or documented implementation conventions; they are not a unique universal formula.
//	Detection needs 1 candle(s). This is a rule-based pattern detector, not a mathematically unique indicator.
//	Heikin-Ashi candles are calculated recursively when applicable.
//
// Reference: https://chartschool.stockcharts.com/table-of-contents/chart-analysis/candlestick-charts/candlestick-pattern-dictionary
// QuantMan's pattern tolerance, trend context and parity have not been verified.
type HeikinashiBullishConfig struct {
	Tolerance      float64
	CandleInterval string
}
type HeikinashiBullish struct {
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

func NewHeikinashiBullish(id string, cfg HeikinashiBullishConfig) (*HeikinashiBullish, error) {
	if id == "" {
		return nil, fmt.Errorf("HeikinashiBullish name cannot be empty")
	}
	if cfg.Tolerance < 0 || cfg.Tolerance >= 1 {
		return nil, fmt.Errorf("tolerance must be in [0,1)")
	}
	return &HeikinashiBullish{id: id, tolerance: cfg.Tolerance, candleInterval: cfg.CandleInterval}, nil
}
func (r *HeikinashiBullish) Update(candle core.MarketCandle) any { r.ingest(candle); return r.result }
func (r *HeikinashiBullish) Result() any                         { return r.result }
func (r *HeikinashiBullish) ingest(candle core.MarketCandle) {
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
	haClose := (c.open + c.high + c.low + c.close) / 4
	haOpen := (c.open + c.close) / 2
	if r.hasHA {
		haOpen = (r.previousHAOpen + r.previousHAClose) / 2
	}
	r.previousHAOpen = haOpen
	r.previousHAClose = haClose
	r.hasHA = true
	c = patternCandle{haOpen, max3(c.high, haOpen, haClose), min3(c.low, haOpen, haClose), haClose}
	r.history = append(r.history, c)
	if len(r.history) > 1 {
		r.history = r.history[1:]
	}
	if len(r.history) < 1 {
		r.result = PatternResult{Timestamp: r.lastTimestamp}
		return
	}
	tol := r.tolerance
	if tol == 0 {
		tol = 0.1
	}
	c = r.history[0]
	detected := c.bull()
	previous := detected
	if r.hasResult {
		previous = r.result.Current
	}
	r.result = PatternResult{IsReady: true, Current: detected, Previous: previous, Timestamp: r.lastTimestamp}
	r.hasResult = true
}
