package derivatives

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: Price Ratio
//
// Standard Formula:
// PriceRatio = FirstInstrumentClose / SecondInstrumentClose
//
// Requires two instrument prices.
// QuantMan exact internal formula not verified.

type PriceRation struct {
	id string

	secondPrice    float64
	hasSecondPrice bool

	lastTime  time.Time
	hasResult bool
	result    PriceRationResult
}

func NewPriceRation(id string, cfg map[string]any) (*PriceRation, error) {
	if id == "" {
		return nil, fmt.Errorf("PriceRation name cannot be empty")
	}

	return &PriceRation{id: id}, nil
}

// Call before Update() for each corresponding candle.
func (r *PriceRation) SetSecondPrice(price float64) {
	r.hasSecondPrice = false

	if price > 0 && !math.IsNaN(price) && !math.IsInf(price, 0) {
		r.secondPrice = price
		r.hasSecondPrice = true
	}
}

func (r *PriceRation) Update(candle core.MarketCandle) any {
	r.ingest(candle)
	return r.result
}

func (r *PriceRation) Result() any {
	return r.result
}

func (r *PriceRation) ingest(candle core.MarketCandle) {
	if candle == nil {
		r.result.IsReady = false
		return
	}

	t := candle.GetTime()
	firstPrice := candle.GetClose()

	if t.IsZero() ||
		(!r.lastTime.IsZero() && !t.After(r.lastTime)) ||
		firstPrice <= 0 ||
		math.IsNaN(firstPrice) ||
		math.IsInf(firstPrice, 0) {
		r.result.IsReady = false
		return
	}

	r.lastTime = t

	if !r.hasSecondPrice {
		r.result = PriceRationResult{
			IsReady:   false,
			Timestamp: t,
		}
		return
	}

	current := firstPrice / r.secondPrice

	previous := current
	if r.hasResult {
		previous = r.result.Current
	}

	r.result = PriceRationResult{
		IsReady:   true,
		Current:   current,
		Previous:  previous,
		Timestamp: t,
	}

	r.hasResult = true

	// Prevent reusing a stale second price.
	r.hasSecondPrice = false
}

var _ core.Indicator = (*PriceRation)(nil)
