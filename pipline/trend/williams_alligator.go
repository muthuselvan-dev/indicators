package trend

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// Indicator Name: Williams Alligator
//
// Pipe Config:
//   type: "williamsAlligator"
//   name: "williamsAlligator"
//   config:
//     jawLength: 13
//     teethLength: 8
//     lipsLength: 5
//     jawOffset: 8
//     teethOffset: 5
//     lipsOffset: 3
//     candleInterval: "1 minutes"
//
// Entry Condition (example):
//   BUY when Lips > Teeth && Teeth > Jaw
//
// Exit Condition (example):
//   SELL when Lips < Teeth && Teeth < Jaw
//
// Original Formula:
//
//   MedianPrice = (High + Low) / 2
//
//   First SMMA = SMA(MedianPrice, Length)
//
//   Next SMMA =
//     (PreviousSMMA * (Length - 1) + MedianPrice)
//     / Length
//
//   Jaw   = SMMA(MedianPrice, jawLength)
//   Teeth = SMMA(MedianPrice, teethLength)
//   Lips  = SMMA(MedianPrice, lipsLength)
//
//   Jaw plotted jawOffset bars forward.
//   Teeth plotted teethOffset bars forward.
//   Lips plotted lipsOffset bars forward.
//
// Result:
//   Jaw, Teeth, Lips, Current, Previous,
//   IsReady, Timestamp
//
// Current = Lips - Teeth (compatibility convention).
// Jaw/Teeth/Lips are unshifted values.
//
// QuantMan condition-level offset behavior is not
// verified by the supplied pipeConfig alone.

type WilliamsAlligatorConfig struct {
	JawLength      int
	TeethLength    int
	LipsLength     int
	JawOffset      int
	TeethOffset    int
	LipsOffset     int
	CandleInterval string
}

type alligatorSMMA struct {
	length int
	count  int
	sum    float64
	value  float64
	ready  bool
}

func (m *alligatorSMMA) update(price float64) {
	m.count++

	if !m.ready {
		m.sum += price

		if m.count < m.length {
			return
		}

		m.value = m.sum / float64(m.length)
		m.ready = true
		return
	}

	m.value = (m.value*float64(m.length-1) + price) / float64(m.length)
}

type WilliamsAlligator struct {
	id  string
	cfg WilliamsAlligatorConfig

	lastTimestamp time.Time

	jaw   alligatorSMMA
	teeth alligatorSMMA
	lips  alligatorSMMA

	hasResult bool
	value     float64

	result WilliamsAlligatorResult
}

func NewWilliamsAlligator(
	id string,
	cfg map[string]any,
) (*WilliamsAlligator, error) {
	if id == "" {
		return nil, fmt.Errorf("indicator id is required")
	}

	config := WilliamsAlligatorConfig{
		JawLength:      13,
		TeethLength:    8,
		LipsLength:     5,
		JawOffset:      8,
		TeethOffset:    5,
		LipsOffset:     3,
		CandleInterval: "1 minutes",
	}

	readInt := func(key string, target *int) error {
		value, exists := cfg[key]
		if !exists {
			return nil
		}

		parsed, ok := value.(int)
		if !ok {
			return fmt.Errorf(
				"williamsAlligator: %s must be an integer",
				key,
			)
		}

		*target = parsed
		return nil
	}

	for _, item := range []struct {
		key    string
		target *int
	}{
		{"jawLength", &config.JawLength},
		{"teethLength", &config.TeethLength},
		{"lipsLength", &config.LipsLength},
		{"jawOffset", &config.JawOffset},
		{"teethOffset", &config.TeethOffset},
		{"lipsOffset", &config.LipsOffset},
	} {
		if err := readInt(item.key, item.target); err != nil {
			return nil, err
		}
	}

	if value, exists := cfg["candleInterval"]; exists {
		interval, ok := value.(string)
		if !ok || interval == "" {
			return nil, fmt.Errorf(
				"williamsAlligator: invalid candleInterval",
			)
		}
		config.CandleInterval = interval
	}

	if config.JawLength <= 0 ||
		config.TeethLength <= 0 ||
		config.LipsLength <= 0 {
		return nil, fmt.Errorf(
			"williamsAlligator: lengths must be positive",
		)
	}

	if config.JawOffset < 0 ||
		config.TeethOffset < 0 ||
		config.LipsOffset < 0 {
		return nil, fmt.Errorf(
			"williamsAlligator: offsets cannot be negative",
		)
	}

	return &WilliamsAlligator{
		id:  id,
		cfg: config,

		jaw: alligatorSMMA{
			length: config.JawLength,
		},
		teeth: alligatorSMMA{
			length: config.TeethLength,
		},
		lips: alligatorSMMA{
			length: config.LipsLength,
		},
	}, nil
}

func (s *WilliamsAlligator) Update(
	candle core.MarketCandle,
) any {
	s.ingest(candle)
	return s.result
}

func (s *WilliamsAlligator) Result() any {
	return s.result
}

func (s *WilliamsAlligator) ingest(
	candle core.MarketCandle,
) {
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

	medianPrice := (high + low) / 2.0

	s.jaw.update(medianPrice)
	s.teeth.update(medianPrice)
	s.lips.update(medianPrice)

	// All three SMMA values must be initialized.
	if !s.jaw.ready ||
		!s.teeth.ready ||
		!s.lips.ready {
		return
	}

	jaw := s.jaw.value
	teeth := s.teeth.value
	lips := s.lips.value

	// Preserve existing single-value convention.
	current := lips - teeth

	if math.IsNaN(current) ||
		math.IsInf(current, 0) {
		return
	}

	previous := current

	if s.hasResult {
		previous = s.value
	}

	s.result.Jaw = jaw
	s.result.Teeth = teeth
	s.result.Lips = lips

	s.result.Current = current
	s.result.Previous = previous
	s.result.Timestamp = timestamp
	s.result.IsReady = true

	s.value = current
	s.hasResult = true
}
