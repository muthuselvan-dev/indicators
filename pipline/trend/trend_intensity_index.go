package trend

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// Indicator Name: Trend Intensity Index (TII)
//
// Pipe Config:
//
//	type: "trendIntensityIndex"
//	name: "trendIntensityIndex"
//	config:
//	  majorLength: 60
//	  minorLength: 30
//	  candleInterval: "1 minutes"
//
// Entry Condition (example):
//
//	BUY: Previous <= 50 && Current > 50
//
// Exit Condition (example):
//
//	SELL: Previous >= 50 && Current < 50
//
// Original Formula (standard TII, per-bar SMA):
//
//	SMA[t]      = Average(Close[t-majorLength+1 .. t])      (one SMA for EACH bar)
//	Positive    = Sum over the last minorLength bars of max(Close[t] - SMA[t], 0)
//	Negative    = Sum over the last minorLength bars of max(SMA[t] - Close[t], 0)
//	TII         = 100 * Positive / (Positive + Negative)
//
// Each of the last minorLength bars is compared with its OWN
// majorLength SMA (the SMA ending on that same bar), exactly like
// sum(close > sma(close, major) ? close - sma : 0, minor) in
// charting platforms.
//
// First result is ready after majorLength + minorLength - 1 closes.
//
// Result:
//
//	Current, Previous, IsReady, Timestamp
//
// Verified against QuantMan (Is Above / Is Below 50, BANKNIFTY 1 min,
// 08 Aug to 28 Aug 2025): 13/13 trades match.

type TrendIntensityIndexConfig struct {
	MajorLength    int
	MinorLength    int
	CandleInterval string
}

type TrendIntensityIndex struct {
	id  string
	cfg TrendIntensityIndexConfig

	closes        []float64
	lastTimestamp time.Time

	hasResult bool
	value     float64
	result    TrendIntensityIndexResult
}

func NewTrendIntensityIndex(id string, cfg map[string]any) (*TrendIntensityIndex, error) {

	if id == "" {
		return nil, fmt.Errorf("indicator id is required")
	}

	config := TrendIntensityIndexConfig{
		MajorLength:    60,
		MinorLength:    30,
		CandleInterval: "1 minutes",
	}

	if v, ok := cfg["majorLength"].(int); ok {
		config.MajorLength = v
	}

	if v, ok := cfg["minorLength"].(int); ok {
		config.MinorLength = v
	}

	if v, ok := cfg["candleInterval"].(string); ok {
		config.CandleInterval = v
	}

	if config.MajorLength <= 0 {
		return nil, fmt.Errorf(
			"trendIntensityIndex: majorLength must be positive",
		)
	}

	if config.MinorLength <= 0 {
		return nil, fmt.Errorf(
			"trendIntensityIndex: minorLength must be positive",
		)
	}

	if config.MinorLength > config.MajorLength {
		return nil, fmt.Errorf(
			"trendIntensityIndex: minorLength cannot exceed majorLength",
		)
	}

	return &TrendIntensityIndex{
		id:  id,
		cfg: config,
	}, nil
}

func (s *TrendIntensityIndex) Update(candle core.MarketCandle) any {
	s.ingest(candle)
	return s.result
}

func (s *TrendIntensityIndex) Result() any {
	return s.result
}

func (s *TrendIntensityIndex) ingest(candle core.MarketCandle) {
	s.result.IsReady = false

	if candle == nil {
		return
	}

	timestamp := candle.GetTime()
	high := candle.GetHigh()
	low := candle.GetLow()
	closePrice := candle.GetClose()

	if timestamp.IsZero() ||
		(!s.lastTimestamp.IsZero() &&
			!timestamp.After(s.lastTimestamp)) ||
		math.IsNaN(high) ||
		math.IsInf(high, 0) ||
		math.IsNaN(low) ||
		math.IsInf(low, 0) ||
		math.IsNaN(closePrice) ||
		math.IsInf(closePrice, 0) ||
		high < low ||
		low <= 0 ||
		closePrice <= 0 {
		return
	}

	s.lastTimestamp = timestamp
	s.result.Timestamp = timestamp

	s.closes = append(s.closes, closePrice)

	major := s.cfg.MajorLength
	minor := s.cfg.MinorLength

	// Each of the last minorLength bars needs its own full
	// majorLength window, so majorLength + minorLength - 1 closes
	// are required.
	keep := major + minor - 1

	if len(s.closes) > keep {
		s.closes = s.closes[1:]
	}

	if len(s.closes) < keep {
		return
	}

	n := len(s.closes)
	first := n - minor // index of the oldest bar in the minor window

	// SMA of the majorLength window that ends on bar `first`.
	var windowSum float64
	for j := first - major + 1; j <= first; j++ {
		windowSum += s.closes[j]
	}

	var positive, negative float64

	for i := first; i < n; i++ {
		if i > first {
			// Slide the window forward by one bar.
			windowSum += s.closes[i] - s.closes[i-major]
		}

		sma := windowSum / float64(major)
		deviation := s.closes[i] - sma

		if deviation > 0 {
			positive += deviation
		} else if deviation < 0 {
			negative -= deviation
		}
	}

	total := positive + negative

	// Neutral value when no deviations exist.
	current := 50.0

	if total > 0 {
		current = 100.0 * positive / total
	}

	if math.IsNaN(current) ||
		math.IsInf(current, 0) {
		return
	}

	previous := current

	if s.hasResult {
		previous = s.value
	}

	s.result.Current = current
	s.result.Previous = previous
	s.result.IsReady = true
	s.result.Timestamp = timestamp

	s.value = current
	s.hasResult = true
}
