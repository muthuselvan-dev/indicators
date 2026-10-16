package aggregate

import (
	"fmt"
	"indicators/core"
)

// OHLC -> candleInterval la build aagum
// Volume -> volumeInterval bucket la separate ah aggregate aagum
// candleInterval kudukala na volumeInterval ah thaan use pannum
type VolumeCandleChart struct {
	candleAgg *Aggregator
	volumeAgg *Aggregator
	lastVol   core.MarketCandle // last completed volume bucket
}

func NewVolumeCandleChart(cfg map[string]any) (*VolumeCandleChart, error) {

	volumeInterval := getString(cfg, "volumeInterval")
	if volumeInterval == "" {
		return nil, fmt.Errorf("volume: volumeInterval is required")
	}

	candleInterval := getString(cfg, "candleInterval")
	if candleInterval == "" {
		candleInterval = volumeInterval
	}

	ca, err := NewAggregatorFromString(candleInterval)
	if err != nil {
		return nil, fmt.Errorf("volume: %w", err)
	}
	va, err := NewAggregatorFromString(volumeInterval)
	if err != nil {
		return nil, fmt.Errorf("volume: %w", err)
	}

	return &VolumeCandleChart{
		candleAgg: ca,
		volumeAgg: va,
	}, nil
}

func (v *VolumeCandleChart) Process(m core.MarketCandle) (core.MarketCandle, bool) {

	if vb, ok := v.volumeAgg.Aggregate(m); ok {
		v.lastVol = vb
	}

	c, ok := v.candleAgg.Aggregate(m)
	if !ok {
		return nil, false
	}

	out := core.Candle{
		Time:         c.GetTime(),
		Open:         c.GetOpen(),
		High:         c.GetHigh(),
		Low:          c.GetLow(),
		Close:        c.GetClose(),
		Volume:       c.GetVolume(),
		OpenInterest: c.GetOpenInterest(),
	}
	if v.lastVol != nil {
		out.Volume = v.lastVol.GetVolume()
	}
	return out, true
}
