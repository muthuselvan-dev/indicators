package technical

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// Indicator: QQE Signals
//
// QuantMan Config:
//   rsiLength:       14
//   smoothingFactor: 5
//   qqeFactor:       4.238
//   candleInterval:  "1 minutes"
//
// Go Pipe Config:
//   Type: "qqeSignals"
//   Name: "qqeSignals"
//   Config: map[string]any{
//       "candleInterval":  "1 minutes",
//       "rsiLength":       14,
//       "smoothingFactor": 5,
//       "qqeFactor":       4.238,
//   }
//
// Standard QQE Calculation:
//
//   1. Wilder RSI(Close, RSILength)
//   2. SmoothedRSI = EMA(RSI, SmoothingFactor)
//   3. RSIChange = Abs(SmoothedRSI - PreviousSmoothedRSI)
//   4. WildersPeriod = 2*RSILength - 1
//   5. ATRRSI = EMA(RSIChange, WildersPeriod)
//        alpha = 2 / (WildersPeriod + 1)  (= 1 / RSILength)
//   6. DAR = EMA(ATRRSI, WildersPeriod) * QQEFactor
//   7. LongBand  = SmoothedRSI - DAR
//   8. ShortBand = SmoothedRSI + DAR
//   9. Apply trailing-band rules
//  10. Determine bullish/bearish trend
//
// Every EMA is seeded with the SMA of its first N values.
//
// Trailing bands:
//   LongBand  = (PrevRSI > PrevLongBand  && RSI > PrevLongBand)
//               ? Max(PrevLongBand, NewLongBand) : NewLongBand
//   ShortBand = (PrevRSI < PrevShortBand && RSI < PrevShortBand)
//               ? Min(PrevShortBand, NewShortBand) : NewShortBand
//
// Trend:
//   RSI crosses above previous ShortBand -> 1  (bullish)
//   RSI crosses below previous LongBand  -> -1 (bearish)
//   otherwise keep previous trend (0 until the first cross)
//
// Available QQE Outputs:
//
//   QQELong
//   QQEShort
//   IsLong
//   IsShort
//
// ScalarResult:
//   Current  = Trend state: 1 bullish, -1 bearish, 0 neutral
//   Previous = Previous ready trend state
//
// Warmup (1 minute candles, defaults): about 72 candles.
//
// Note:
//   Standard QQE interpretation.
//   QuantMan output mapping and exact parity are unverified.

type QuantitativeQualitativeEstimationSignalsConfig struct {
	RSILength       int
	SmoothingFactor int
	QQEFactor       float64
	CandleInterval  string
}

type QuantitativeQualitativeEstimationSignals struct {
	id  string
	cfg QuantitativeQualitativeEstimationSignalsConfig

	lastTime time.Time

	previousClose float64
	hasPrevious   bool

	gainSum     float64
	lossSum     float64
	changeCount int
	averageGain float64
	averageLoss float64
	rsiReady    bool

	rsiSeedSum   float64
	rsiSeedCount int
	smoothedRSI  float64
	emaReady     bool

	previousSmoothedRSI float64
	hasPreviousRSI      bool

	atrRSISum   float64
	atrRSICount int
	atrRSI      float64
	atrReady    bool

	darSum   float64
	darCount int
	dar      float64
	darReady bool

	longBand  float64
	shortBand float64
	trend     int
	bandReady bool

	// Smoothed RSI of the previous published candle.
	signalRSI float64

	lastTrend float64
	hasResult bool

	result      ScalarResult
	signalState QQESignalsResult
}

func NewQuantitativeQualitativeEstimationSignals(id string, cfg map[string]any) (*QuantitativeQualitativeEstimationSignals, error) {
	if id == "" {
		return nil, fmt.Errorf("qqeSignals: indicator id is required")
	}

	config := QuantitativeQualitativeEstimationSignalsConfig{
		RSILength:       14,
		SmoothingFactor: 5,
		QQEFactor:       4.238,
		CandleInterval:  "1 minutes",
	}

	if value, exists := cfg["rsiLength"]; exists {
		v, ok := value.(int)
		if !ok {
			return nil, fmt.Errorf("qqeSignals: rsiLength must be an integer")
		}
		config.RSILength = v
	}

	if value, exists := cfg["smoothingFactor"]; exists {
		v, ok := value.(int)
		if !ok {
			return nil, fmt.Errorf("qqeSignals: smoothingFactor must be an integer")
		}
		config.SmoothingFactor = v
	}

	if value, exists := cfg["qqeFactor"]; exists {
		switch v := value.(type) {
		case float64:
			config.QQEFactor = v
		case int:
			config.QQEFactor = float64(v)
		default:
			return nil, fmt.Errorf("qqeSignals: qqeFactor must be numeric")
		}
	}

	if value, exists := cfg["candleInterval"]; exists {
		v, ok := value.(string)
		if !ok || v == "" {
			return nil, fmt.Errorf("qqeSignals: invalid candleInterval")
		}
		config.CandleInterval = v
	}

	if config.RSILength < 2 ||
		config.SmoothingFactor < 1 ||
		math.IsNaN(config.QQEFactor) ||
		math.IsInf(config.QQEFactor, 0) ||
		config.QQEFactor <= 0 {
		return nil, fmt.Errorf("qqeSignals: invalid configuration")
	}

	return &QuantitativeQualitativeEstimationSignals{
		id:  id,
		cfg: config,
	}, nil
}

func (r *QuantitativeQualitativeEstimationSignals) Update(candle core.MarketCandle) any {
	r.ingest(candle)
	return r.result
}

func (r *QuantitativeQualitativeEstimationSignals) Result() any {
	return r.result
}

// Signals exposes the four proposed QQE outputs.
func (r *QuantitativeQualitativeEstimationSignals) Signals() QQESignalsResult {
	return r.signalState
}

func (r *QuantitativeQualitativeEstimationSignals) ingest(candle core.MarketCandle) {
	if !validQQESignalsCandle(candle, r.lastTime) {
		r.result.IsReady = false
		r.signalState.IsReady = false
		return
	}

	timestamp := candle.GetTime()
	closePrice := candle.GetClose()
	r.lastTime = timestamp

	if !r.hasPrevious {
		r.previousClose = closePrice
		r.hasPrevious = true
		r.setNotReady(timestamp)
		return
	}

	change := closePrice - r.previousClose
	r.previousClose = closePrice

	gain := math.Max(change, 0)
	loss := math.Max(-change, 0)

	// Step 1: Wilder RSI.
	if !r.rsiReady {
		r.gainSum += gain
		r.lossSum += loss
		r.changeCount++

		if r.changeCount < r.cfg.RSILength {
			r.setNotReady(timestamp)
			return
		}

		period := float64(r.cfg.RSILength)
		r.averageGain = r.gainSum / period
		r.averageLoss = r.lossSum / period
		r.rsiReady = true
	} else {
		period := float64(r.cfg.RSILength)

		r.averageGain = (r.averageGain*(period-1) + gain) / period
		r.averageLoss = (r.averageLoss*(period-1) + loss) / period
	}

	rsi := 50.0

	if r.averageLoss == 0 {
		if r.averageGain > 0 {
			rsi = 100
		}
	} else {
		rs := r.averageGain / r.averageLoss
		rsi = 100 - 100/(1+rs)
	}

	// Step 2: EMA smoothing of RSI.
	if !r.emaReady {
		r.rsiSeedSum += rsi
		r.rsiSeedCount++

		if r.rsiSeedCount < r.cfg.SmoothingFactor {
			r.setNotReady(timestamp)
			return
		}

		r.smoothedRSI = r.rsiSeedSum / float64(r.cfg.SmoothingFactor)
		r.emaReady = true
	} else {
		alpha := 2.0 / float64(r.cfg.SmoothingFactor+1)

		r.smoothedRSI = alpha*rsi + (1-alpha)*r.smoothedRSI
	}

	// Step 3: Smoothed RSI absolute change.
	if !r.hasPreviousRSI {
		r.previousSmoothedRSI = r.smoothedRSI
		r.hasPreviousRSI = true
		r.setNotReady(timestamp)
		return
	}

	rsiChange := math.Abs(r.smoothedRSI - r.previousSmoothedRSI)

	r.previousSmoothedRSI = r.smoothedRSI

	wildersPeriod := 2*r.cfg.RSILength - 1
	wildersLength := float64(wildersPeriod)

	// EMA alpha for WildersPeriod (equals 1 / RSILength).
	alphaW := 2.0 / (wildersLength + 1)

	// Step 4: First smoothing (EMA of RSI change).
	if !r.atrReady {
		r.atrRSISum += rsiChange
		r.atrRSICount++

		if r.atrRSICount < wildersPeriod {
			r.setNotReady(timestamp)
			return
		}

		r.atrRSI = r.atrRSISum / wildersLength
		r.atrReady = true
	} else {
		r.atrRSI = alphaW*rsiChange + (1-alphaW)*r.atrRSI
	}

	// Step 5: Second smoothing (EMA of ATR RSI).
	if !r.darReady {
		r.darSum += r.atrRSI
		r.darCount++

		if r.darCount < wildersPeriod {
			r.setNotReady(timestamp)
			return
		}

		r.dar = r.darSum / wildersLength
		r.darReady = true
	} else {
		r.dar = alphaW*r.atrRSI + (1-alphaW)*r.dar
	}

	// Step 6: Dynamic average range.
	dynamicRange := r.dar * r.cfg.QQEFactor

	newLongBand := r.smoothedRSI - dynamicRange
	newShortBand := r.smoothedRSI + dynamicRange

	// Step 7: Initialize trailing bands.
	if !r.bandReady {
		r.longBand = newLongBand
		r.shortBand = newShortBand
		r.bandReady = true

		r.publish(timestamp)
		return
	}

	previousLongBand := r.longBand
	previousShortBand := r.shortBand

	// Smoothed RSI of the previous published candle.
	previousRSI := r.signalRSI

	// Step 8: Trailing long band.
	if previousRSI > previousLongBand &&
		r.smoothedRSI > previousLongBand {
		r.longBand = math.Max(previousLongBand, newLongBand)
	} else {
		r.longBand = newLongBand
	}

	// Step 9: Trailing short band.
	if previousRSI < previousShortBand &&
		r.smoothedRSI < previousShortBand {
		r.shortBand = math.Min(previousShortBand, newShortBand)
	} else {
		r.shortBand = newShortBand
	}

	// Step 10: Determine trend.
	if previousRSI <= previousShortBand &&
		r.smoothedRSI > previousShortBand {
		r.trend = 1
	} else if previousRSI >= previousLongBand &&
		r.smoothedRSI < previousLongBand {
		r.trend = -1
	}

	r.publish(timestamp)
}

func (r *QuantitativeQualitativeEstimationSignals) publish(timestamp time.Time) {
	current := float64(r.trend)
	previous := current

	if r.hasResult {
		previous = r.lastTrend
	}

	r.result = ScalarResult{
		IsReady:   true,
		Current:   current,
		Previous:  previous,
		Timestamp: timestamp,
	}

	r.signalState = QQESignalsResult{
		QQELong:   r.longBand,
		QQEShort:  r.shortBand,
		IsLong:    r.trend == 1,
		IsShort:   r.trend == -1,
		IsReady:   true,
		Timestamp: timestamp,
	}

	r.signalRSI = r.smoothedRSI
	r.lastTrend = current
	r.hasResult = true
}

func (r *QuantitativeQualitativeEstimationSignals) setNotReady(timestamp time.Time) {
	r.result = ScalarResult{
		IsReady:   false,
		Timestamp: timestamp,
	}

	r.signalState = QQESignalsResult{
		IsReady:   false,
		Timestamp: timestamp,
	}
}

func validQQESignalsCandle(candle core.MarketCandle, lastTimestamp time.Time) bool {
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
