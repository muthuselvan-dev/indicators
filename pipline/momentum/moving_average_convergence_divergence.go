package momentum

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: Moving Average Convergence Divergence (MACD)
//
// pipeConfigs: [
//
//	{
//	  type: 'movingAverageConvergenceDivergence',
//	  name: 'macd',
//	  config: {
//	    fastPeriod: 12,
//	    slowPeriod: 26,
//	    signalPeriod: 9,
//	    candleInterval: '1 minutes',
//	  }
//	}
//
// ],
//
// Available result:
//
//   - MACD Line
//   - Signal Line
//   - MACD Histogram
//
// Entry Condition:
//
//   MACD Line Crosses Above Signal Line
//   Previous <= PreviousSignal && Current > Signal
//
// Exit Condition:
//
//   MACD Line Crosses Below Signal Line
//   Previous >= PreviousSignal && Current < Signal
//
// Original Formula:
//
// EMA = (CurrentClose * Multiplier) +
//       (PreviousEMA * (1 - Multiplier))
//
// Multiplier = 2 / (period + 1)
//
// Initial EMA = SMA(Close, period)
//
// FastEMA = EMA(Close, fastPeriod)
// SlowEMA = EMA(Close, slowPeriod)
//
// MACDLine = FastEMA - SlowEMA
//
// SignalLine = EMA(MACDLine, signalPeriod)
//
// Histogram = MACDLine - SignalLine
//
// Current represents current MACD Line.
// Previous represents previous MACD Line.
//
// Signal represents current Signal Line.
// PreviousSignal represents previous Signal Line.
//
// Histogram represents current MACD Histogram.
// PreviousHistogram represents previous Histogram.
//
// First complete MACD result is ready after
// slowPeriod + signalPeriod - 1 valid closing prices.
//
// All periods are configurable.
// EMA calculations are independent within this indicator.

type MACD struct {
	id string

	fastPeriod     int
	slowPeriod     int
	signalPeriod   int
	candleInterval string

	candleCount int

	fastSum float64
	slowSum float64

	fastEMA float64
	slowEMA float64

	signalCount int
	signalSum   float64
	signalEMA   float64

	hasResult bool

	lastMACD      float64
	lastSignal    float64
	lastHistogram float64

	lastTimestamp time.Time

	result MACDResult
}

func NewMACD(id string, cfg map[string]any) (*MACD, error) {
	fastPeriod, ok := cfg["fastPeriod"].(int)
	if !ok {
		return nil, fmt.Errorf("MACD fastPeriod must be an int")
	}

	slowPeriod, ok := cfg["slowPeriod"].(int)
	if !ok {
		return nil, fmt.Errorf("MACD slowPeriod must be an int")
	}

	signalPeriod, ok := cfg["signalPeriod"].(int)
	if !ok {
		return nil, fmt.Errorf("MACD signalPeriod must be an int")
	}

	if id == "" {
		return nil, fmt.Errorf("MACD name cannot be empty")
	}

	if fastPeriod <= 0 || slowPeriod <= 0 || signalPeriod <= 0 {
		return nil, fmt.Errorf("MACD periods must be positive")
	}

	if fastPeriod >= slowPeriod {
		return nil, fmt.Errorf("MACD fastPeriod must be smaller than slowPeriod")
	}

	return &MACD{
		id:             id,
		fastPeriod:     fastPeriod,
		slowPeriod:     slowPeriod,
		signalPeriod:   signalPeriod,
		candleInterval: cfg["candleInterval"].(string),
	}, nil
}

func (m *MACD) Update(candle core.MarketCandle) any {
	m.ingest(candle)
	return m.result
}

func (m *MACD) Result() any {
	return m.result
}

func (m *MACD) ingest(candle core.MarketCandle) {
	if candle == nil {
		return
	}

	timestamp := candle.GetTime()
	closePrice := candle.GetClose()

	// Reject invalid, duplicate or out-of-order candles.
	if timestamp.IsZero() ||
		math.IsNaN(closePrice) ||
		math.IsInf(closePrice, 0) ||
		closePrice <= 0 ||
		(!m.lastTimestamp.IsZero() &&
			!timestamp.After(m.lastTimestamp)) {

		m.setNotReady(timestamp)
		return
	}

	m.lastTimestamp = timestamp
	m.candleCount++

	// Initialize fast EMA with SMA.
	if m.candleCount <= m.fastPeriod {
		m.fastSum += closePrice

		if m.candleCount == m.fastPeriod {
			m.fastEMA = m.fastSum / float64(m.fastPeriod)
		}
	} else {
		// Update fast EMA.
		multiplier := 2.0 / float64(m.fastPeriod+1)
		m.fastEMA = (closePrice * multiplier) +
			(m.fastEMA * (1 - multiplier))
	}

	// Initialize slow EMA with SMA.
	if m.candleCount <= m.slowPeriod {
		m.slowSum += closePrice

		if m.candleCount == m.slowPeriod {
			m.slowEMA = m.slowSum / float64(m.slowPeriod)
		}
	} else {
		// Update slow EMA.
		multiplier := 2.0 / float64(m.slowPeriod+1)
		m.slowEMA = (closePrice * multiplier) +
			(m.slowEMA * (1 - multiplier))
	}

	// MACD requires both EMA values.
	if m.candleCount < m.slowPeriod {
		m.setNotReady(timestamp)
		return
	}

	// Calculate MACD Line.
	currentMACD := m.fastEMA - m.slowEMA

	if math.IsNaN(currentMACD) || math.IsInf(currentMACD, 0) {
		m.setNotReady(timestamp)
		return
	}

	// Initialize Signal EMA with SMA of MACD values.
	m.signalCount++

	if m.signalCount <= m.signalPeriod {
		m.signalSum += currentMACD

		if m.signalCount == m.signalPeriod {
			m.signalEMA = m.signalSum / float64(m.signalPeriod)
		}
	} else {
		// Update Signal EMA.
		multiplier := 2.0 / float64(m.signalPeriod+1)
		m.signalEMA = (currentMACD * multiplier) +
			(m.signalEMA * (1 - multiplier))
	}

	// Wait until Signal EMA is initialized.
	if m.signalCount < m.signalPeriod {
		m.setNotReady(timestamp)
		return
	}

	currentSignal := m.signalEMA
	currentHistogram := currentMACD - currentSignal

	if math.IsNaN(currentSignal) ||
		math.IsInf(currentSignal, 0) ||
		math.IsNaN(currentHistogram) ||
		math.IsInf(currentHistogram, 0) {
		m.setNotReady(timestamp)
		return
	}

	previousMACD := currentMACD
	previousSignal := currentSignal
	previousHistogram := currentHistogram

	if m.hasResult {
		previousMACD = m.lastMACD
		previousSignal = m.lastSignal
		previousHistogram = m.lastHistogram
	}

	m.result = MACDResult{
		IsReady:           true,
		Current:           currentMACD,
		Previous:          previousMACD,
		Signal:            currentSignal,
		PreviousSignal:    previousSignal,
		Histogram:         currentHistogram,
		PreviousHistogram: previousHistogram,
		Timestamp:         timestamp,
	}

	m.lastMACD = currentMACD
	m.lastSignal = currentSignal
	m.lastHistogram = currentHistogram
	m.hasResult = true
}

func (m *MACD) setNotReady(timestamp time.Time) {
	m.result = MACDResult{
		IsReady:   false,
		Timestamp: timestamp,
	}
}
