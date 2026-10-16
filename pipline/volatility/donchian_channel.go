package volatility

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// Indicator Name: Donchian Channel
//
// QuantMan Pipe Config:
//
// {
//     Type: "donChianChannel",
//     Name: "donChianChannel",
//     Config: map[string]any{
//         "length":         20,
//         "lag":            false,
//         "candleInterval": "1 minutes",
//     },
// }
//
// Formula:
//
//   Upper  = HighestHigh(Length)
//   Lower  = LowestLow(Length)
//   Middle = (Upper + Lower) / 2
//
// Lag:
//
//   lag = false : the channel INCLUDES the current candle.
//                 Close can never be above the Upper band (Close <= High
//                 <= Upper), so "Close Is Above Upper" never triggers.
//                 Verified: QuantMan returns "No entry signals" for
//                 Close Is Above Upper with lag = false.
//
//   lag = true  : the channel is shifted by one candle: the Upper/Lower/
//                 Middle reported on candle t are the channel of candle
//                 t-1 (window = the Length candles before the current one).
//                 This makes breakouts possible.
//                 NOTE: this lag semantics is an assumption, not yet
//                 verified against a QuantMan lag = true run.
//
// Entry Condition (QuantMan):
//   Current Close Is Above Don Chian Channel Upper:  Close > Upper
//
// Exit Condition (QuantMan):
//   Current Close Is Below Don Chian Channel Lower:  Close < Lower
//
// Result:
//
//   Upper
//   Lower
//   Middle
//
//   BaseResult:
//     Current   = Middle
//     Previous  = Previous ready Middle
//     IsReady
//     Timestamp
//
// Warmup:
//   lag = false : first ready result requires Length valid candles.
//   lag = true  : first ready result requires Length + 1 valid candles.

type DonchianChannelConfig struct {
	Length         int
	Lag            bool
	CandleInterval string
}

type DonchianChannel struct {
	id  string
	cfg DonchianChannelConfig

	highs []float64
	lows  []float64

	lastTime time.Time

	// Channel of the previous candle (used when Lag = true).
	hasPrevChannel bool
	prevUpper      float64
	prevLower      float64
	prevMiddle     float64

	// Previous ready reported Middle.
	hasResult bool
	lastValue float64

	result DonchianChannelResult
}

func NewDonchianChannel(id string, cfg map[string]any) (*DonchianChannel, error) {
	if id == "" {
		return nil, fmt.Errorf("donChianChannel: indicator id is required")
	}

	config := DonchianChannelConfig{
		Length:         20,
		Lag:            false,
		CandleInterval: "1 minutes",
	}

	if value, exists := cfg["length"]; exists {
		length, ok := value.(int)
		if !ok {
			return nil, fmt.Errorf("donChianChannel: length must be an integer")
		}
		config.Length = length
	}

	if value, exists := cfg["lag"]; exists {
		lag, ok := value.(bool)
		if !ok {
			return nil, fmt.Errorf("donChianChannel: lag must be boolean")
		}
		config.Lag = lag
	}

	if value, exists := cfg["candleInterval"]; exists {
		interval, ok := value.(string)
		if !ok || interval == "" {
			return nil, fmt.Errorf("donChianChannel: invalid candleInterval")
		}
		config.CandleInterval = interval
	}

	if config.Length <= 0 {
		return nil, fmt.Errorf("donChianChannel: length must be positive")
	}

	return &DonchianChannel{
		id:    id,
		cfg:   config,
		highs: make([]float64, 0, config.Length),
		lows:  make([]float64, 0, config.Length),
	}, nil
}

func (d *DonchianChannel) Update(candle core.MarketCandle) any {
	d.ingest(candle)
	return d.result
}

func (d *DonchianChannel) Result() any {
	return d.result
}

func (d *DonchianChannel) notReady(timestamp time.Time) {
	d.result = DonchianChannelResult{
		BaseResult: BaseResult{
			IsReady:   false,
			Timestamp: timestamp,
		},
	}
}

func (d *DonchianChannel) ingest(candle core.MarketCandle) {
	if !validDonchianChannelCandle(candle, d.lastTime) {
		d.result.IsReady = false
		return
	}

	timestamp := candle.GetTime()
	high := candle.GetHigh()
	low := candle.GetLow()

	d.lastTime = timestamp

	// Step 1: Maintain rolling High and Low.
	d.highs = append(d.highs, high)
	d.lows = append(d.lows, low)

	if len(d.highs) > d.cfg.Length {
		d.highs = d.highs[1:]
		d.lows = d.lows[1:]
	}

	// Step 2: Wait for N valid candles.
	if len(d.highs) < d.cfg.Length {
		d.notReady(timestamp)
		return
	}

	// Step 3: Channel of the window ending at the current candle.
	upper := d.highs[0]
	lower := d.lows[0]

	for i := 1; i < len(d.highs); i++ {
		upper = math.Max(upper, d.highs[i])
		lower = math.Min(lower, d.lows[i])
	}

	middle := (upper + lower) / 2

	// Step 4: Lag. Report the previous candle's channel.
	outUpper, outLower, outMiddle := upper, lower, middle

	if d.cfg.Lag {
		if !d.hasPrevChannel {
			d.prevUpper, d.prevLower, d.prevMiddle = upper, lower, middle
			d.hasPrevChannel = true
			d.notReady(timestamp)
			return
		}

		outUpper, outLower, outMiddle = d.prevUpper, d.prevLower, d.prevMiddle
		d.prevUpper, d.prevLower, d.prevMiddle = upper, lower, middle
	}

	// Step 5: Previous ready Middle.
	previous := outMiddle

	if d.hasResult {
		previous = d.lastValue
	}

	// Step 6: Update result.
	d.result = DonchianChannelResult{
		BaseResult: BaseResult{
			IsReady:   true,
			Current:   outMiddle,
			Previous:  previous,
			Timestamp: timestamp,
		},
		Upper:  outUpper,
		Lower:  outLower,
		Middle: outMiddle,
	}

	d.lastValue = outMiddle
	d.hasResult = true
}

func validDonchianChannelCandle(candle core.MarketCandle, lastTimestamp time.Time) bool {
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
		if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
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
