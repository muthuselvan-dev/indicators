package pipline

import (
	"indicators/core"
)

type Rule struct {
	Type   string         `json:"type"`
	Name   string         `json:"name"`
	Config map[string]any `json:"config"`
}

type MarketCandle = core.MarketCandle
type Indicator = core.Indicator
