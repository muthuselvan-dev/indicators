package aggregate

import (
	"fmt"
	"indicators/core"
	"strings"
)

// Chart: ella chart type-um ithe implement pannum
type Chart interface {
	Process(c core.MarketCandle) (core.MarketCandle, bool)
}

func NewChart(cfg map[string]any) (Chart, error) {
	chartType, _ := cfg["type"].(string)

	switch strings.ToLower(strings.TrimSpace(chartType)) {
	case "candle":
		return NewCandleChart(cfg)
	case "heikinashi":
		return NewHeikinAshiChart(cfg)
	case "volume", "volumecandle":
		return NewVolumeCandleChart(cfg)
	case "":
		return nil, fmt.Errorf("chart type is required")
	default:
		return nil, fmt.Errorf("invalid chart type %q", chartType)
	}
}

// getString: cfg la irundhu string eduka (common helper)
func getString(cfg map[string]any, key string) string {
	s, _ := cfg[key].(string)
	return strings.TrimSpace(s)
}
