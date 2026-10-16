package volatility

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// Indicator Name: Mean Reversion Channel (MRC)
//
// QuantMan Pipe Config:
//
// {
//     Type: "meanReversionChannel",
//     Name: "mrc",
//     Config: map[string]any{
//         "period":          200,
//         "innerMultiplier": 1.0,
//         "outerMultiplier": 2.415,
//         "filterType":      "superSmoother",
//         "candleInterval":  "1 minutes",
//     },
// }
//
// Filter Type Options:
//
//   superSmoother - Ehlers Super Smoother
//   ehlersEma     - Ehlers EMA
//   gaussian      - Gaussian Filter
//   butterWorth   - Butterworth Filter
//   bandStop      - Band Stop Filter
//   sma           - Simple Moving Average
//   ema           - Exponential Moving Average
//   rma           - Wilder Moving Average
//
// Implemented Filter:
//   superSmoother only.
//
// Reference Calculation:
//
//   Middle = TwoPoleSuperSmoother(Close, Period)
//
//   TrueRange = Max(
//       High - Low,
//       Abs(High - PreviousClose),
//       Abs(Low - PreviousClose)
//   )
//
//   Width = TwoPoleSuperSmoother(TrueRange, Period)
//
//   InnerUpper = Middle + InnerMultiplier * Width
//   InnerLower = Middle - InnerMultiplier * Width
//
//   OuterUpper = Middle + OuterMultiplier * Width
//   OuterLower = Middle - OuterMultiplier * Width
//
// Result:
//
//   BaseResult:
//     Current  = Middle
//     Previous = Previous ready Middle
//     IsReady
//     Timestamp
//
//   Middle
//   Upper
//   Lower
//   InnerUpper
//   InnerLower
//   OuterUpper
//   OuterLower
//   ChannelWidth
//
// Warmup:
//   Period valid candles.
//
// Important:
//   This is a reference implementation.
//   QuantMan's exact MRC width, initialization,
//   and filter semantics are not verified.

type MeanReversionChannelConfig struct {
	Period          int
	InnerMultiplier float64
	OuterMultiplier float64
	FilterType      string
	CandleInterval  string
}

type MeanReversionChannelResult struct {
	BaseResult

	Middle       float64
	Upper        float64
	Lower        float64
	InnerUpper   float64
	InnerLower   float64
	OuterUpper   float64
	OuterLower   float64
	ChannelWidth float64
}

type MeanReversionChannel struct {
	id  string
	cfg MeanReversionChannelConfig

	count int

	previousClose float64
	hasClose      bool

	pricePrevious1 float64
	pricePrevious2 float64
	priceFilter1   float64
	priceFilter2   float64

	rangePrevious1 float64
	rangePrevious2 float64
	rangeFilter1   float64
	rangeFilter2   float64

	lastTime time.Time

	hasResult bool
	lastValue float64

	result MeanReversionChannelResult
}

func NewMeanReversionChannel(id string, cfg map[string]any) (*MeanReversionChannel, error) {
	if id == "" {
		return nil, fmt.Errorf("meanReversionChannel: indicator id is required")
	}

	config := MeanReversionChannelConfig{
		Period:          200,
		InnerMultiplier: 1.0,
		OuterMultiplier: 2.415,
		FilterType:      "superSmoother",
		CandleInterval:  "1 minutes",
	}

	if value, exists := cfg["period"]; exists {
		period, ok := value.(int)
		if !ok {
			return nil, fmt.Errorf("meanReversionChannel: period must be integer")
		}
		config.Period = period
	}

	if value, exists := cfg["innerMultiplier"]; exists {
		switch v := value.(type) {
		case int:
			config.InnerMultiplier = float64(v)
		case float64:
			config.InnerMultiplier = v
		default:
			return nil, fmt.Errorf("meanReversionChannel: innerMultiplier must be numeric")
		}
	}

	if value, exists := cfg["outerMultiplier"]; exists {
		switch v := value.(type) {
		case int:
			config.OuterMultiplier = float64(v)
		case float64:
			config.OuterMultiplier = v
		default:
			return nil, fmt.Errorf("meanReversionChannel: outerMultiplier must be numeric")
		}
	}

	if value, exists := cfg["filterType"]; exists {
		filterType, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("meanReversionChannel: filterType must be string")
		}
		config.FilterType = filterType
	}

	if value, exists := cfg["candleInterval"]; exists {
		interval, ok := value.(string)
		if !ok || interval == "" {
			return nil, fmt.Errorf("meanReversionChannel: invalid candleInterval")
		}
		config.CandleInterval = interval
	}

	if config.Period < 3 {
		return nil, fmt.Errorf("meanReversionChannel: period must be at least 3")
	}

	if math.IsNaN(config.InnerMultiplier) ||
		math.IsInf(config.InnerMultiplier, 0) ||
		config.InnerMultiplier < 0 {
		return nil, fmt.Errorf("meanReversionChannel: invalid innerMultiplier")
	}

	if math.IsNaN(config.OuterMultiplier) ||
		math.IsInf(config.OuterMultiplier, 0) ||
		config.OuterMultiplier < config.InnerMultiplier {
		return nil, fmt.Errorf("meanReversionChannel: invalid outerMultiplier")
	}

	if config.FilterType != "superSmoother" {
		return nil, fmt.Errorf(
			"meanReversionChannel: filterType %q is not implemented",
			config.FilterType,
		)
	}

	return &MeanReversionChannel{
		id:  id,
		cfg: config,
	}, nil
}

func (r *MeanReversionChannel) Update(candle core.MarketCandle) any {
	r.ingest(candle)
	return r.result
}

func (r *MeanReversionChannel) Result() any {
	return r.result
}

func (r *MeanReversionChannel) ingest(candle core.MarketCandle) {
	if !validMeanReversionChannelCandle(candle, r.lastTime) {
		r.result.IsReady = false
		return
	}

	timestamp := candle.GetTime()
	high := candle.GetHigh()
	low := candle.GetLow()
	closePrice := candle.GetClose()

	// Step 1: Calculate True Range.
	trueRange := high - low

	if r.hasClose {
		trueRange = math.Max(
			trueRange,
			math.Max(
				math.Abs(high-r.previousClose),
				math.Abs(low-r.previousClose),
			),
		)
	}

	r.previousClose = closePrice
	r.hasClose = true
	r.lastTime = timestamp
	r.count++

	// Step 2: Calculate Super Smoother coefficients.
	period := float64(r.cfg.Period)

	a1 := math.Exp(-math.Sqrt2 * math.Pi / period)
	b1 := 2 * a1 * math.Cos(math.Sqrt2*math.Pi/period)

	c2 := b1
	c3 := -a1 * a1
	c1 := 1 - c2 - c3

	// Step 3: Initialize filter state.
	if r.count == 1 {
		r.pricePrevious1 = closePrice
		r.pricePrevious2 = closePrice
		r.priceFilter1 = closePrice
		r.priceFilter2 = closePrice

		r.rangePrevious1 = trueRange
		r.rangePrevious2 = trueRange
		r.rangeFilter1 = trueRange
		r.rangeFilter2 = trueRange
	}

	// Step 4: Calculate filtered Middle and Width.
	middle := closePrice
	width := trueRange

	if r.count > 1 {
		middle = c1*(closePrice+r.pricePrevious1)/2 +
			c2*r.priceFilter1 +
			c3*r.priceFilter2

		width = c1*(trueRange+r.rangePrevious1)/2 +
			c2*r.rangeFilter1 +
			c3*r.rangeFilter2
	}

	// Step 5: Update recursive filter state.
	r.pricePrevious2 = r.pricePrevious1
	r.pricePrevious1 = closePrice

	r.priceFilter2 = r.priceFilter1
	r.priceFilter1 = middle

	r.rangePrevious2 = r.rangePrevious1
	r.rangePrevious1 = trueRange

	r.rangeFilter2 = r.rangeFilter1
	r.rangeFilter1 = width

	// Step 6: Wait for configured warmup.
	if r.count < r.cfg.Period {
		r.result = MeanReversionChannelResult{
			BaseResult: BaseResult{
				IsReady:   false,
				Timestamp: timestamp,
			},
		}
		return
	}

	// Step 7: Calculate channel bands.
	innerUpper := middle + r.cfg.InnerMultiplier*width
	innerLower := middle - r.cfg.InnerMultiplier*width

	outerUpper := middle + r.cfg.OuterMultiplier*width
	outerLower := middle - r.cfg.OuterMultiplier*width

	// Step 8: Validate calculated results.
	for _, value := range []float64{
		middle, width,
		innerUpper, innerLower,
		outerUpper, outerLower,
	} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			r.result.IsReady = false
			return
		}
	}

	// Step 9: Previous ready Middle.
	previous := middle

	if r.hasResult {
		previous = r.lastValue
	}

	// Step 10: Update result.
	r.result = MeanReversionChannelResult{
		BaseResult: BaseResult{
			IsReady:   true,
			Current:   middle,
			Previous:  previous,
			Timestamp: timestamp,
		},
		Middle:       middle,
		Upper:        outerUpper,
		Lower:        outerLower,
		InnerUpper:   innerUpper,
		InnerLower:   innerLower,
		OuterUpper:   outerUpper,
		OuterLower:   outerLower,
		ChannelWidth: width,
	}

	r.lastValue = middle
	r.hasResult = true
}

func validMeanReversionChannelCandle(candle core.MarketCandle, lastTimestamp time.Time) bool {
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

var _ core.Indicator = (*MeanReversionChannel)(nil)
