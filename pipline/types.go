package pipline

import (
	"indicators/core"
)

type Rule struct {
	ChartConfig map[string]any `json:"chartConfig"`
	Type        string         `json:"type"`
	Name        string         `json:"name"`
	Config      map[string]any `json:"config"`
}

type MarketCandle = core.MarketCandle
type Indicator = core.Indicator
