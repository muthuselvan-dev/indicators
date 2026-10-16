package volatility

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// Indicator Name: Keltner Channel
//
// QuantMan Pipe Config:
//
// {
//     Type: "keltnerChannel",
//     Name: "keltnerChannel",
//     Config: map[string]any{
//         "candleInterval": "1 minutes",
//         "length":         20,
//         "multiplier":     2,
//         "atrLength":      10,
//     },
// }
//
// Original Formula:
//
//   EMA Seed = SMA(First Length Closes)
//
//   EMA Alpha = 2 / (Length + 1)
//
//   EMA = Alpha * Close + (1 - Alpha) * PreviousEMA
//
//   TrueRange = Max(
//       High - Low,
//       Abs(High - PreviousClose),
//       Abs(Low - PreviousClose)
//   )
//
//   ATR Seed = Sum(First ATRLength TrueRanges) / ATRLength
//
//   Wilder ATR =
//       (PreviousATR * (ATRLength - 1) + TrueRange) / ATRLength
//
//   Middle = EMA(Close, Length)
//
//   Upper = Middle + Multiplier * ATR
//
//   Lower = Middle - Multiplier * ATR
//
// Entry Condition:
//
//   Close Is Above Upper Band:
//     Close > Upper
//
//   Close Crosses Above Upper Band:
//     PreviousClose <= PreviousUpper && Close > Upper
//
// Exit Condition:
//
//   Close Is Below Lower Band:
//     Close < Lower
//
//   Close Crosses Below Lower Band:
//     PreviousClose >= PreviousLower && Close < Lower
//
// Result:
//
//   Middle
//   Upper
//   Lower
//
//   BaseResult:
//     Current   = Middle
//     Previous  = Previous ready Middle
//     IsReady
//     Timestamp
//
// Warmup:
//   EMA requires Length valid closes.
//   ATR requires ATRLength valid candles.
//   First candle TR = High - Low.
//   First ready = Max(Length, ATRLength) candles.
//
// Note:
//   QuantMan exact calculation parity is not verified.

type KeltnerChannelConfig struct {
	Length         int
	ATRLength      int
	Multiplier     float64
	CandleInterval string
}

type KeltnerChannelResult struct {
	BaseResult
	Middle float64
	Upper  float64
	Lower  float64
}

type KeltnerChannel struct {
	id  string
	cfg KeltnerChannelConfig

	closeSum   float64
	closeCount int
	ema        float64

	trSum   float64
	trCount int
	atr     float64

	previousClose float64
	hasClose      bool

	lastTime time.Time

	hasResult bool
	lastValue float64

	result KeltnerChannelResult
}

func NewKeltnerChannel(id string, cfg map[string]any) (*KeltnerChannel, error) {
	if id == "" {
		return nil, fmt.Errorf("keltnerChannel: indicator id is required")
	}

	config := KeltnerChannelConfig{
		Length:         20,
		ATRLength:      10,
		Multiplier:     2.0,
		CandleInterval: "1 minutes",
	}

	if value, exists := cfg["length"]; exists {
		length, ok := value.(int)
		if !ok {
			return nil, fmt.Errorf("keltnerChannel: length must be an integer")
		}
		config.Length = length
	}

	if value, exists := cfg["atrLength"]; exists {
		atrLength, ok := value.(int)
		if !ok {
			return nil, fmt.Errorf("keltnerChannel: atrLength must be an integer")
		}
		config.ATRLength = atrLength
	}

	if value, exists := cfg["multiplier"]; exists {
		switch v := value.(type) {
		case int:
			config.Multiplier = float64(v)
		case float64:
			config.Multiplier = v
		default:
			return nil, fmt.Errorf("keltnerChannel: multiplier must be numeric")
		}
	}

	if value, exists := cfg["candleInterval"]; exists {
		interval, ok := value.(string)
		if !ok || interval == "" {
			return nil, fmt.Errorf("keltnerChannel: invalid candleInterval")
		}
		config.CandleInterval = interval
	}

	if config.Length <= 0 {
		return nil, fmt.Errorf("keltnerChannel: length must be positive")
	}

	if config.ATRLength <= 0 {
		return nil, fmt.Errorf("keltnerChannel: atrLength must be positive")
	}

	if math.IsNaN(config.Multiplier) ||
		math.IsInf(config.Multiplier, 0) ||
		config.Multiplier <= 0 {
		return nil, fmt.Errorf("keltnerChannel: multiplier must be positive and finite")
	}

	return &KeltnerChannel{
		id:  id,
		cfg: config,
	}, nil
}

func (k *KeltnerChannel) Update(candle core.MarketCandle) any {
	k.ingest(candle)
	return k.result
}

func (k *KeltnerChannel) Result() any {
	return k.result
}

func (k *KeltnerChannel) ingest(candle core.MarketCandle) {
	if !validKeltnerChannelCandle(candle, k.lastTime) {
		k.result.IsReady = false
		return
	}

	timestamp := candle.GetTime()
	high := candle.GetHigh()
	low := candle.GetLow()
	closePrice := candle.GetClose()

	// Step 1: Calculate True Range.
	trueRange := high - low

	if k.hasClose {
		trueRange = math.Max(
			trueRange,
			math.Max(
				math.Abs(high-k.previousClose),
				math.Abs(low-k.previousClose),
			),
		)
	}

	k.previousClose = closePrice
	k.hasClose = true
	k.lastTime = timestamp

	// Step 2: EMA calculation with SMA seed.
	if k.closeCount < k.cfg.Length {
		k.closeSum += closePrice
		k.closeCount++

		if k.closeCount == k.cfg.Length {
			k.ema = k.closeSum / float64(k.cfg.Length)
		}
	} else {
		alpha := 2.0 / float64(k.cfg.Length+1)
		k.ema = alpha*closePrice + (1-alpha)*k.ema
	}

	// Step 3: ATR calculation with Wilder smoothing.
	if k.trCount < k.cfg.ATRLength {
		k.trSum += trueRange
		k.trCount++

		if k.trCount == k.cfg.ATRLength {
			k.atr = k.trSum / float64(k.cfg.ATRLength)
		}
	} else {
		k.atr = (k.atr*float64(k.cfg.ATRLength-1) + trueRange) /
			float64(k.cfg.ATRLength)
	}

	// Step 4: Wait until EMA and ATR are ready.
	if k.closeCount < k.cfg.Length ||
		k.trCount < k.cfg.ATRLength {
		k.result = KeltnerChannelResult{
			BaseResult: BaseResult{
				IsReady:   false,
				Timestamp: timestamp,
			},
		}
		return
	}

	// Step 5: Calculate Keltner Channel.
	middle := k.ema
	upper := middle + k.cfg.Multiplier*k.atr
	lower := middle - k.cfg.Multiplier*k.atr

	// Step 6: Validate calculated values.
	for _, value := range []float64{middle, upper, lower} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			k.result.IsReady = false
			return
		}
	}

	// Step 7: Previous ready Middle.
	previous := middle

	if k.hasResult {
		previous = k.lastValue
	}

	// Step 8: Update result.
	k.result = KeltnerChannelResult{
		BaseResult: BaseResult{
			IsReady:   true,
			Current:   middle,
			Previous:  previous,
			Timestamp: timestamp,
		},
		Middle: middle,
		Upper:  upper,
		Lower:  lower,
	}

	k.lastValue = middle
	k.hasResult = true
}

func validKeltnerChannelCandle(candle core.MarketCandle, lastTimestamp time.Time) bool {
	if candle == nil {
		return false
	}

	timestamp := candle.GetTime()

	if timestamp.IsZero() {
		return false
	}

	// Reject duplicate or out-of-order candles.
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
