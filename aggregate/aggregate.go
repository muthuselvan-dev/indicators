package aggregate

import (
	"fmt"
	"indicators/core"
	"strconv"
	"strings"
	"time"
)

type Aggregator struct {
	chart ChartProcessor
}

func NewAggregator(config map[string]any) (*Aggregator, error) {

	if config == nil {
		return nil, fmt.Errorf("aggregate: config cannot be nil")
	}

	chartType, ok := config["type"].(string)
	if !ok || chartType == "" {
		return nil, fmt.Errorf("aggregate: type required")
	}

	candleInterval, ok := config["candleInterval"].(string)

	if !ok || strings.TrimSpace(candleInterval) == "" {
		return nil, fmt.Errorf("aggregate: candleInterval required")
	}

	if chartType == "volumeCandle" {
		volumeInterval, ok := config["volumeInterval"].(string)
		if !ok || strings.TrimSpace(volumeInterval) == "" {
			return nil, fmt.Errorf("aggregate: volumeInterval required")
		}
	}

	var chart ChartProcessor
	var err error

	switch chartType {
	case "candle":
		chart, err = NewCandleChart("candle", config)

	case "heikinashi":
		chart, err = NewHeikinChart("heikinashi", config)

	case "volumeCandle":
		chart, err = NewVolumeChart("volumeCandle", config)

	default:
		return nil, fmt.Errorf("aggregate: unsupported chart type %q", chartType)
	}

	if err != nil {
		return nil, err
	}

	return &Aggregator{
		chart: chart,
	}, nil
}

func (a *Aggregator) Aggregate(candle core.MarketCandle) (core.MarketCandle, bool) {
	if a == nil || a.chart == nil || candle == nil {
		return nil, false
	}

	return a.chart.Update(candle)
}

func ParseInterval(value string) (time.Duration, error) {
	value = strings.TrimSpace(strings.ToLower(value))

	if value == "" {
		return 0, fmt.Errorf("interval cannot be empty")
	}

	parts := strings.Fields(value)

	if len(parts) == 1 {
		d, err := time.ParseDuration(value)
		if err != nil || d <= 0 {
			return 0, fmt.Errorf("invalid interval %q", value)
		}
		return d, nil
	}

	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid interval format %q", value)
	}

	number, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || number <= 0 {
		return 0, fmt.Errorf("invalid interval value %q", value)
	}

	var unit time.Duration

	switch parts[1] {
	case "s", "sec", "second", "seconds":
		unit = time.Second

	case "m", "min", "minute", "minutes":
		unit = time.Minute

	case "h", "hr", "hour", "hours":
		unit = time.Hour

	case "d", "day", "days":
		unit = 24 * time.Hour

	default:
		return 0, fmt.Errorf("unsupported interval unit %q", parts[1])
	}

	if number > int64((1<<63-1)/unit) {
		return 0, fmt.Errorf("interval too large %q", value)
	}

	return time.Duration(number) * unit, nil
}
