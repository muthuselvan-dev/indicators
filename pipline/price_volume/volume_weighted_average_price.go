package price_volume

import (
	"fmt"
	"indicators/core"
	"math"
	"strings"
	"time"
)

// describe: Volume Weighted Average Price (VWAP)
//
// QuantMan:
// type: volumeWeightedAveragePrice
// name: vwap
// config: {price: "typical"}
//
// Original Formula:
// Typical Price = (High + Low + Close) / 3
// VWAP = Sum(Price * Volume) / Sum(Volume)
//
// Supported price:
// typical, close, open, high, low, hl2, hlc3, ohlc4
//
// Session:
// Resets when the candle's calendar date changes.
//
// Warmup:
// First candle with positive cumulative volume.
//
// Result:
// ScalarResult from price_volume/type.go.
//
// Note:
// QuantMan exact parity requires backtest verification.

type VolumeWeightedAveragePriceConfig struct {
	Price          string
	CandleInterval string
}

type VolumeWeightedAveragePrice struct {
	id             string
	cfg            VolumeWeightedAveragePriceConfig
	sessionDate    string
	priceVolumeSum float64
	volumeSum      float64
	lastTime       time.Time
	hasResult      bool
	result         ScalarResult
}

func NewVolumeWeightedAveragePrice(id string, cfg map[string]any) (*VolumeWeightedAveragePrice, error) {
	if id == "" {
		return nil, fmt.Errorf("VWAP: name cannot be empty")
	}

	price := "typical"
	candleInterval := "1 minutes"

	if cfg != nil {
		if value, exists := cfg["price"]; exists {
			p, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("VWAP: price must be a string")
			}

			p = strings.ToLower(strings.TrimSpace(p))
			if p != "" {
				price = p
			}
		}

		if value, exists := cfg["candleInterval"]; exists {
			interval, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("VWAP: candleInterval must be a string")
			}

			interval = strings.TrimSpace(interval)
			if interval != "" {
				candleInterval = interval
			}
		}
	}

	switch price {
	case "typical", "hlc3", "close", "open", "high", "low", "hl2", "ohlc4":
	default:
		return nil, fmt.Errorf("VWAP: unsupported price %q", price)
	}

	return &VolumeWeightedAveragePrice{
		id: id,
		cfg: VolumeWeightedAveragePriceConfig{
			Price:          price,
			CandleInterval: candleInterval,
		},
	}, nil
}

func (v *VolumeWeightedAveragePrice) Update(c core.MarketCandle) any {
	v.ingest(c)
	return v.result
}

func (v *VolumeWeightedAveragePrice) Result() any {
	return v.result
}

func (v *VolumeWeightedAveragePrice) ingest(c core.MarketCandle) {
	if !validVolumeWeightedAveragePriceCandle(c, v.lastTime) {
		return
	}

	t := c.GetTime()
	o := c.GetOpen()
	h := c.GetHigh()
	l := c.GetLow()
	cl := c.GetClose()
	volume := c.GetVolume()

	price := v.calculatePrice(o, h, l, cl)

	if !vwapFinite(price) {
		return
	}

	weightedPrice := price * volume

	if !vwapFinite(weightedPrice) {
		return
	}

	day := t.Format("2006-01-02")
	newSession := day != v.sessionDate

	priceVolumeSum := v.priceVolumeSum
	volumeSum := v.volumeSum

	if newSession {
		priceVolumeSum = 0
		volumeSum = 0
	}

	nextPriceVolumeSum := priceVolumeSum + weightedPrice
	nextVolumeSum := volumeSum + volume

	if !vwapFinite(nextPriceVolumeSum) ||
		!vwapFinite(nextVolumeSum) {
		return
	}

	if nextVolumeSum <= 0 {
		v.lastTime = t

		if newSession {
			v.sessionDate = day
			v.hasResult = false
		}

		v.priceVolumeSum = nextPriceVolumeSum
		v.volumeSum = nextVolumeSum
		v.result = ScalarResult{Timestamp: t}
		return
	}

	current := nextPriceVolumeSum / nextVolumeSum

	if !vwapFinite(current) {
		return
	}

	previous := current

	if !newSession && v.hasResult {
		previous = v.result.Current
	}

	v.lastTime = t
	v.sessionDate = day
	v.priceVolumeSum = nextPriceVolumeSum
	v.volumeSum = nextVolumeSum
	v.hasResult = true

	v.result = ScalarResult{
		IsReady:   true,
		Current:   current,
		Previous:  previous,
		Timestamp: t,
	}
}

func (v *VolumeWeightedAveragePrice) calculatePrice(o, h, l, cl float64) float64 {
	switch v.cfg.Price {
	case "open":
		return o
	case "high":
		return h
	case "low":
		return l
	case "close":
		return cl
	case "hl2":
		return (h + l) / 2
	case "ohlc4":
		return (o + h + l + cl) / 4
	case "typical", "hlc3":
		return (h + l + cl) / 3
	default:
		return math.NaN()
	}
}

func vwapFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func validVolumeWeightedAveragePriceCandle(c core.MarketCandle, lastTime time.Time) bool {
	if c == nil {
		return false
	}

	t := c.GetTime()

	if t.IsZero() || (!lastTime.IsZero() && !t.After(lastTime)) {
		return false
	}

	o := c.GetOpen()
	h := c.GetHigh()
	l := c.GetLow()
	cl := c.GetClose()
	volume := c.GetVolume()

	if !vwapFinite(o) || !vwapFinite(h) ||
		!vwapFinite(l) || !vwapFinite(cl) ||
		!vwapFinite(volume) {
		return false
	}

	if o <= 0 || h <= 0 || l <= 0 || cl <= 0 || volume < 0 {
		return false
	}

	if h < l || h < math.Max(o, cl) || l > math.Min(o, cl) {
		return false
	}

	return true
}
