package trend

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// HalfTrend Indicator
//
// QuantMan pipeConfig:
//
// {
//     type: "halfTrend",
//     name: "halfTrend",
//     config: {
//         amplitude: 2,
//         channelDeviation: 2,
//         period: 100,
//         candleInterval: "1 minutes"
//     }
// }
//
// Calculation (original HalfTrend / Pine logic):
//   1. Highest High / Lowest Low over Amplitude
//   2. SMA High / SMA Low over Amplitude
//   3. Stateful trend reversal
//   4. HalfTrend line (on a trend flip the new line starts
//      from the last value of the opposite line)
//   5. ATR-based upper/lower channels
//
// Result:
//   Current
//   Previous
//   IsReady
//   Timestamp

type HalfTrendConfig struct {
	Amplitude        int
	ChannelDeviation float64
	NoOfCandles      int
	CandleInterval   string
}

type HalfTrend struct {
	id  string
	cfg HalfTrendConfig

	highs  []float64
	lows   []float64
	closes []float64

	lastTimestamp time.Time

	// Trend:
	// 0 = uptrend
	// 1 = downtrend
	trend     int
	nextTrend int

	maxLowPrice  float64
	minHighPrice float64

	// true after the first amplitude window initialised the state.
	stateInit bool

	// ATR state
	atrCount int
	atrSum   float64
	atr      float64
	atrReady bool

	// HalfTrend line state
	upLine   float64
	downLine float64

	hasResult bool
	value     float64

	result HalfTrendResult
}

func NewHalfTrend(id string, cfg map[string]any) (*HalfTrend, error) {
	if id == "" {
		return nil, fmt.Errorf("indicator id is required")
	}

	amplitude := 2
	channelDeviation := 2.0
	period := 100
	candleInterval := "1 minutes"

	if value, ok := cfg["amplitude"].(int); ok {
		amplitude = value
	}

	if value, ok := cfg["channelDeviation"].(int); ok {
		channelDeviation = float64(value)
	} else if value, ok := cfg["channelDeviation"].(float64); ok {
		channelDeviation = value
	}

	if value, ok := cfg["period"].(int); ok {
		period = value
	}

	if value, ok := cfg["candleInterval"].(string); ok {
		candleInterval = value
	}

	if amplitude <= 0 {
		return nil, fmt.Errorf(
			"HalfTrend: amplitude must be positive",
		)
	}

	if period <= 0 {
		return nil, fmt.Errorf(
			"HalfTrend: period must be positive",
		)
	}

	return &HalfTrend{
		id: id,
		cfg: HalfTrendConfig{
			Amplitude:        amplitude,
			ChannelDeviation: channelDeviation,
			NoOfCandles:      period,
			CandleInterval:   candleInterval,
		},
		trend:     0,
		nextTrend: 0,
	}, nil
}

func (s *HalfTrend) Update(
	candle core.MarketCandle,
) any {
	s.ingest(candle)
	return s.result
}

func (s *HalfTrend) Result() any {
	return s.result
}

func (s *HalfTrend) ingest(
	candle core.MarketCandle,
) {
	if candle == nil {
		return
	}

	timestamp := candle.GetTime()
	high := candle.GetHigh()
	low := candle.GetLow()
	closePrice := candle.GetClose()

	// Validate OHLC and timestamp.
	if timestamp.IsZero() ||
		(!s.lastTimestamp.IsZero() &&
			!timestamp.After(s.lastTimestamp)) ||
		math.IsNaN(high) ||
		math.IsInf(high, 0) ||
		math.IsNaN(low) ||
		math.IsInf(low, 0) ||
		math.IsNaN(closePrice) ||
		math.IsInf(closePrice, 0) ||
		low <= 0 ||
		high < low ||
		closePrice <= 0 {

		s.result.IsReady = false
		return
	}

	// Previous close is needed for True Range.
	previousClose := closePrice

	if len(s.closes) > 0 {
		previousClose = s.closes[len(s.closes)-1]
	}

	s.lastTimestamp = timestamp
	s.result.Timestamp = timestamp
	s.result.IsReady = false

	s.highs = append(s.highs, high)
	s.lows = append(s.lows, low)
	s.closes = append(s.closes, closePrice)

	amplitude := s.cfg.Amplitude
	atrPeriod := s.cfg.NoOfCandles

	// Retain sufficient history for both calculations.
	maxHistory := amplitude

	if atrPeriod > maxHistory {
		maxHistory = atrPeriod
	}

	maxHistory++

	if len(s.highs) > maxHistory {
		s.highs = s.highs[1:]
		s.lows = s.lows[1:]
		s.closes = s.closes[1:]
	}

	// True Range
	tr := math.Max(
		high-low,
		math.Max(
			math.Abs(high-previousClose),
			math.Abs(low-previousClose),
		),
	)

	// Wilder ATR initialization and smoothing.
	if !s.atrReady {
		s.atrSum += tr
		s.atrCount++

		if s.atrCount >= atrPeriod {
			s.atr = s.atrSum / float64(atrPeriod)
			s.atrReady = true
		}
	} else {
		s.atr = (s.atr*float64(atrPeriod-1) + tr) / float64(atrPeriod)
	}

	if len(s.highs) < amplitude {
		return
	}

	start := len(s.highs) - amplitude

	highestHigh := s.highs[start]
	lowestLow := s.lows[start]

	var highSum float64
	var lowSum float64

	for i := start; i < len(s.highs); i++ {
		h := s.highs[i]
		l := s.lows[i]

		highestHigh = math.Max(highestHigh, h)
		lowestLow = math.Min(lowestLow, l)

		highSum += h
		lowSum += l
	}

	highMA := highSum / float64(amplitude)
	lowMA := lowSum / float64(amplitude)

	previousHigh := high
	previousLow := low

	if len(s.highs) >= 2 {
		previousHigh = s.highs[len(s.highs)-2]
		previousLow = s.lows[len(s.lows)-2]
	}

	// Initialize state once, on the first calculated window
	// (original: maxLowPrice = low[1], minHighPrice = high[1]).
	if !s.stateInit {
		s.maxLowPrice = previousLow
		s.minHighPrice = previousHigh
		s.upLine = s.maxLowPrice
		s.downLine = s.minHighPrice
		s.stateInit = true
	}

	// Trend before this candle (needed to detect a flip).
	previousTrend := s.trend

	// Stateful HalfTrend reversal.
	if s.nextTrend == 1 {
		s.maxLowPrice = math.Max(
			lowestLow,
			s.maxLowPrice,
		)

		if highMA < s.maxLowPrice &&
			closePrice < previousLow {

			s.trend = 1
			s.nextTrend = 0
			s.minHighPrice = highestHigh
		}
	} else {
		s.minHighPrice = math.Min(
			highestHigh,
			s.minHighPrice,
		)

		if lowMA > s.minHighPrice &&
			closePrice > previousHigh {

			s.trend = 0
			s.nextTrend = 1
			s.maxLowPrice = lowestLow
		}
	}

	// HalfTrend line.
	var current float64

	if s.trend == 0 {
		if previousTrend != 0 {
			// Just flipped to uptrend: continue from the last down line.
			s.upLine = s.downLine
		} else {
			s.upLine = math.Max(
				s.maxLowPrice,
				s.upLine,
			)
		}

		current = s.upLine
	} else {
		if previousTrend != 1 {
			// Just flipped to downtrend: continue from the last up line.
			s.downLine = s.upLine
		} else {
			s.downLine = math.Min(
				s.minHighPrice,
				s.downLine,
			)
		}

		current = s.downLine
	}

	// HalfTrend ATR channel.
	// Channel values are calculated independently
	// of the core HalfTrend line.
	if s.atrReady {
		channelWidth := s.cfg.ChannelDeviation *
			(s.atr / 2.0)

		upperChannel := current + channelWidth
		lowerChannel := current - channelWidth

		_ = upperChannel
		_ = lowerChannel
	}

	if !s.atrReady {
		return
	}

	if math.IsNaN(current) || math.IsInf(current, 0) {
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
