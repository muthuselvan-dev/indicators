package aggregate

import (
	"fmt"
	"indicators/core"
)

type CandleChart struct {
	agg *Aggregator
}

func NewCandleChart(cfg map[string]any) (*CandleChart, error) {

	candleInterval := getString(cfg, "candleInterval")

	if candleInterval == "" {
		return nil, fmt.Errorf("candle: candleInterval is required")
	}

	agg, err := NewAggregatorFromString(candleInterval)
	if err != nil {
		return nil, fmt.Errorf("candle: %w", err)
	}
	return &CandleChart{
		agg: agg,
	}, nil
}

func (c *CandleChart) Process(m core.MarketCandle) (core.MarketCandle, bool) {
	return c.agg.Aggregate(m)
}
