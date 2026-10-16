package trend

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// Average Directional Index (ADX)
//
// QuantMan Type: averageDirectionalIndex
//
// Config:
//   DILength       : 14
//   ADXSmoothing   : 14
//   candleInterval : "1 minutes"
//
// Wilder Formula:
//
// TR = max(
//     High - Low,
//     abs(High - PreviousClose),
//     abs(Low - PreviousClose),
// )
//
// +DM = High - PreviousHigh
//       if upward movement dominates and is positive
//
// -DM = PreviousLow - Low
//       if downward movement dominates and is positive
//
// +DI = 100 * Smoothed(+DM) / Smoothed(TR)
// -DI = 100 * Smoothed(-DM) / Smoothed(TR)
//
// DX = 100 * abs(+DI - -DI) / (+DI + -DI)
//
// ADX = Wilder RMA(DX, ADXSmoothing)
//
// Result:
//   Current   = ADX
//   Previous  = Previous ADX
//   PlusDI    = Positive Directional Indicator
//   MinusDI   = Negative Directional Indicator
//   IsReady
//   Timestamp

type ADXConfig struct {
	DILength       int
	ADXSmoothing   int
	CandleInterval string
}

type ADX struct {
	id  string
	cfg ADXConfig

	lastTimestamp time.Time

	// Previous candle values
	hasPreviousCandle bool
	previousHigh      float64
	previousLow       float64
	previousClose     float64

	// Initial DI smoothing
	diCount  int
	trSum    float64
	plusSum  float64
	minusSum float64

	// Wilder smoothed values
	smoothedTR      float64
	smoothedPlusDM  float64
	smoothedMinusDM float64
	diReady         bool

	// Initial ADX smoothing
	dxCount int
	dxSum   float64

	// Previous ready ADX
	hasResult bool
	value     float64

	result ADXResult
}

func NewADX(id string, cfg map[string]any) (*ADX, error) {
	if id == "" {
		return nil, fmt.Errorf("indicator id is required")
	}

	diLength := 14
	adxSmoothing := 14
	candleInterval := "1 minutes"

	if value, ok := cfg["DILength"].(int); ok {
		diLength = value
	}

	if value, ok := cfg["ADXSmoothing"].(int); ok {
		adxSmoothing = value
	}

	if value, ok := cfg["candleInterval"].(string); ok {
		candleInterval = value
	}

	return &ADX{
		id: id,
		cfg: ADXConfig{
			DILength:       diLength,
			ADXSmoothing:   adxSmoothing,
			CandleInterval: candleInterval,
		},
	}, nil
}

func (s *ADX) Update(candle core.MarketCandle) any {
	s.ingest(candle)
	return s.result
}

func (s *ADX) Result() any {
	return s.result
}

func (s *ADX) ingest(candle core.MarketCandle) {
	if candle == nil {
		return
	}

	timestamp := candle.GetTime()
	high := candle.GetHigh()
	low := candle.GetLow()
	closePrice := candle.GetClose()

	// Candle validation
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

	s.lastTimestamp = timestamp
	s.result.Timestamp = timestamp
	s.result.IsReady = false

	// First candle seeds previous OHLC.
	if !s.hasPreviousCandle {
		s.previousHigh = high
		s.previousLow = low
		s.previousClose = closePrice
		s.hasPreviousCandle = true
		return
	}

	// True Range
	tr := math.Max(
		high-low,
		math.Max(
			math.Abs(high-s.previousClose),
			math.Abs(low-s.previousClose),
		),
	)

	// Directional Movement
	upMove := high - s.previousHigh
	downMove := s.previousLow - low

	plusDM := 0.0
	minusDM := 0.0

	if upMove > downMove && upMove > 0 {
		plusDM = upMove
	}

	if downMove > upMove && downMove > 0 {
		minusDM = downMove
	}

	// Update previous OHLC for next candle.
	s.previousHigh = high
	s.previousLow = low
	s.previousClose = closePrice

	diLength := s.cfg.DILength
	adxSmoothing := s.cfg.ADXSmoothing

	// Initialize Wilder DI smoothing using first
	// DILength TR and DM observations.
	if !s.diReady {
		s.trSum += tr
		s.plusSum += plusDM
		s.minusSum += minusDM
		s.diCount++

		if s.diCount < diLength {
			return
		}

		s.smoothedTR = s.trSum
		s.smoothedPlusDM = s.plusSum
		s.smoothedMinusDM = s.minusSum
		s.diReady = true
	} else {
		// Wilder smoothing
		s.smoothedTR = s.smoothedTR -
			s.smoothedTR/float64(diLength) + tr

		s.smoothedPlusDM = s.smoothedPlusDM -
			s.smoothedPlusDM/float64(diLength) + plusDM

		s.smoothedMinusDM = s.smoothedMinusDM -
			s.smoothedMinusDM/float64(diLength) + minusDM
	}

	// Directional Indicators
	plusDI := 0.0
	minusDI := 0.0

	if s.smoothedTR > 0 {
		plusDI = 100 *
			s.smoothedPlusDM / s.smoothedTR

		minusDI = 100 *
			s.smoothedMinusDM / s.smoothedTR
	}

	// Directional Index
	dx := 0.0
	diTotal := plusDI + minusDI

	if diTotal > 0 {
		dx = 100 *
			math.Abs(plusDI-minusDI) / diTotal
	}

	s.result.PlusDI = plusDI
	s.result.MinusDI = minusDI

	// First ADXSmoothing DX values:
	// seed ADX with their arithmetic mean.
	if !s.hasResult {
		s.dxSum += dx
		s.dxCount++

		if s.dxCount < adxSmoothing {
			return
		}

		current := s.dxSum / float64(adxSmoothing)

		s.result.Current = current
		s.result.Previous = current
		s.result.IsReady = true

		s.value = current
		s.hasResult = true
		return
	}

	// Subsequent ADX:
	// Wilder recursive moving average.
	current := (s.value*float64(adxSmoothing-1) + dx) / float64(adxSmoothing)

	s.result.Current = current
	s.result.Previous = s.value
	s.result.IsReady = true

	s.value = current
}
