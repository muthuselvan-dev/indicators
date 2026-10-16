package technical

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// Indicator: Moving Average For RSI
//
// QuantMan Original Config:
//   noOfCandlesForRSI: 14
//   noOfCandlesForMA: 9
//   candleInterval: "1 minutes"
//
// Go Pipe Config:
//   Type: "movingAverageForRSI"
//   Name: "movingAverageForRSI"
//   Config: map[string]any{
//       "rsiPeriod":      14,
//       "maPeriod":       9,
//       "candleInterval": "1 minutes",
//   }
//
// Formula:
//
//   Change = Close - PreviousClose
//
//   Gain = Max(Change, 0)
//   Loss = Max(-Change, 0)
//
//   InitialAverageGain = SMA(Gain, RSIPeriod)
//   InitialAverageLoss = SMA(Loss, RSIPeriod)
//
//   WilderAverageGain =
//       (PreviousAverageGain * (RSIPeriod - 1) + Gain) / RSIPeriod
//
//   WilderAverageLoss =
//       (PreviousAverageLoss * (RSIPeriod - 1) + Loss) / RSIPeriod
//
//   RS = AverageGain / AverageLoss
//
//   RSI = 100 - 100 / (1 + RS)
//
//   MovingAverageForRSI = SMA(RSI, MAPeriod)
//
// Warmup:
//   RSIPeriod + MAPeriod valid candles.
//
// Result:
//   Current  = SMA of RSI
//   Previous = Previous ready SMA of RSI
//   IsReady
//   Timestamp
//
// Note:
//   Standard Wilder RSI with SMA smoothing.
//   QuantMan exact calculation parity is not yet verified.

type MovingAverageForRelativeStrengthIndexConfig struct {
	RSIPeriod      int
	MAPeriod       int
	CandleInterval string
}

type MovingAverageForRelativeStrengthIndex struct {
	id  string
	cfg MovingAverageForRelativeStrengthIndexConfig

	previousClose float64
	hasPrevious   bool

	gainSum float64
	lossSum float64

	averageGain float64
	averageLoss float64

	changeCount int
	rsiReady    bool

	rsiValues []float64
	rsiSum    float64

	lastTime time.Time

	hasResult bool
	lastValue float64

	result ScalarResult
}

func NewMovingAverageForRelativeStrengthIndex(id string, cfg map[string]any) (*MovingAverageForRelativeStrengthIndex, error) {
	if id == "" {
		return nil, fmt.Errorf("movingAverageForRSI: indicator id is required")
	}

	config := MovingAverageForRelativeStrengthIndexConfig{
		RSIPeriod:      14,
		MAPeriod:       9,
		CandleInterval: "1 minutes",
	}

	if value, exists := cfg["rsiPeriod"]; exists {
		period, ok := value.(int)
		if !ok {
			return nil, fmt.Errorf("movingAverageForRSI: rsiPeriod must be an integer")
		}
		config.RSIPeriod = period
	}

	if value, exists := cfg["maPeriod"]; exists {
		period, ok := value.(int)
		if !ok {
			return nil, fmt.Errorf("movingAverageForRSI: maPeriod must be an integer")
		}
		config.MAPeriod = period
	}

	if value, exists := cfg["candleInterval"]; exists {
		interval, ok := value.(string)
		if !ok || interval == "" {
			return nil, fmt.Errorf("movingAverageForRSI: invalid candleInterval")
		}
		config.CandleInterval = interval
	}

	if config.RSIPeriod <= 0 {
		return nil, fmt.Errorf("movingAverageForRSI: rsiPeriod must be positive")
	}

	if config.MAPeriod <= 0 {
		return nil, fmt.Errorf("movingAverageForRSI: maPeriod must be positive")
	}

	return &MovingAverageForRelativeStrengthIndex{
		id:        id,
		cfg:       config,
		rsiValues: make([]float64, 0, config.MAPeriod),
	}, nil
}

func (r *MovingAverageForRelativeStrengthIndex) Update(candle core.MarketCandle) any {
	r.ingest(candle)
	return r.result
}

func (r *MovingAverageForRelativeStrengthIndex) Result() any {
	return r.result
}

func (r *MovingAverageForRelativeStrengthIndex) ingest(candle core.MarketCandle) {
	if !validMovingAverageForRSICandle(candle, r.lastTime) {
		r.result.IsReady = false
		return
	}

	timestamp := candle.GetTime()
	closePrice := candle.GetClose()

	r.lastTime = timestamp

	// Step 1: First candle initializes previous close.
	if !r.hasPrevious {
		r.previousClose = closePrice
		r.hasPrevious = true

		r.result = ScalarResult{
			IsReady:   false,
			Timestamp: timestamp,
		}
		return
	}

	// Step 2: Calculate price change.
	change := closePrice - r.previousClose
	r.previousClose = closePrice

	gain := math.Max(change, 0)
	loss := math.Max(-change, 0)

	// Step 3: Initialize Wilder RSI using SMA.
	if !r.rsiReady {
		r.gainSum += gain
		r.lossSum += loss
		r.changeCount++

		if r.changeCount < r.cfg.RSIPeriod {
			r.result = ScalarResult{
				IsReady:   false,
				Timestamp: timestamp,
			}
			return
		}

		r.averageGain = r.gainSum / float64(r.cfg.RSIPeriod)
		r.averageLoss = r.lossSum / float64(r.cfg.RSIPeriod)

		r.rsiReady = true
	} else {
		// Step 4: Wilder recursive smoothing.
		period := float64(r.cfg.RSIPeriod)

		r.averageGain = (r.averageGain*(period-1) + gain) / period

		r.averageLoss = (r.averageLoss*(period-1) + loss) / period
	}

	// Step 5: Calculate RSI.
	rsi := 50.0

	if r.averageLoss == 0 {
		if r.averageGain > 0 {
			rsi = 100
		}
	} else {
		rs := r.averageGain / r.averageLoss
		rsi = 100 - 100/(1+rs)
	}

	// Step 6: Maintain RSI moving-average window.
	r.rsiValues = append(r.rsiValues, rsi)
	r.rsiSum += rsi

	if len(r.rsiValues) > r.cfg.MAPeriod {
		r.rsiSum -= r.rsiValues[0]
		r.rsiValues = r.rsiValues[1:]
	}

	// Step 7: Wait for MA warmup.
	if len(r.rsiValues) < r.cfg.MAPeriod {
		r.result = ScalarResult{
			IsReady:   false,
			Timestamp: timestamp,
		}
		return
	}

	// Step 8: Calculate SMA of RSI.
	current := r.rsiSum / float64(r.cfg.MAPeriod)

	if math.IsNaN(current) || math.IsInf(current, 0) {
		r.result = ScalarResult{
			IsReady:   false,
			Timestamp: timestamp,
		}
		return
	}

	// Step 9: Previous ready value.
	previous := current

	if r.hasResult {
		previous = r.lastValue
	}

	// Step 10: Store result.
	r.result = ScalarResult{
		IsReady:   true,
		Current:   current,
		Previous:  previous,
		Timestamp: timestamp,
	}

	r.lastValue = current
	r.hasResult = true
}

func validMovingAverageForRSICandle(candle core.MarketCandle, lastTimestamp time.Time) bool {
	if candle == nil {
		return false
	}

	timestamp := candle.GetTime()

	if timestamp.IsZero() {
		return false
	}

	if !lastTimestamp.IsZero() && !timestamp.After(lastTimestamp) {
		return false
	}

	open := candle.GetOpen()
	high := candle.GetHigh()
	low := candle.GetLow()
	closePrice := candle.GetClose()

	for _, value := range []float64{open, high, low, closePrice} {
		if math.IsNaN(value) ||
			math.IsInf(value, 0) ||
			value <= 0 {
			return false
		}
	}

	if high < low {
		return false
	}

	if high < math.Max(open, closePrice) {
		return false
	}

	if low > math.Min(open, closePrice) {
		return false
	}

	return true
}
