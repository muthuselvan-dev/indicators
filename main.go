package main

import (
	"log"

	"indicators/condition"
	"indicators/core/datasource"
	"indicators/pipline"
	"indicators/strategy"
)

func main() {
	// Indicator Pipeline Configuration:
	// Add all your indicators to this list (RSI, CCI, and any future indicators):
	pipeConfigs := []pipline.Rule{
		{
			Type: "relativeStrengthIndex",
			Name: "rsi",
			Config: map[string]any{
				"period":         14,
				"candleInterval": "1 minutes",
			},
		},

		{
			Type: "commodityChannelIndex",
			Name: "commodityChannelIndex",
			Config: map[string]any{
				"period":         20,
				"candleInterval": "1 minutes",
			},
		},
		{
			Type: "awesomeOscillator",
			Name: "awesomeOscillator",
			Config: map[string]any{
				"candleInterval": "1 minutes",
				"fastPeriod":     5,
				"slowPeriod":     34,
			},
		},
	}

	pl, err := pipline.NewPipeline(pipeConfigs)
	if err != nil {
		log.Fatal(err)
	}

	candles, err := datasource.ReadCSV("output.csv")
	if err != nil {
		log.Fatal(err)
	}

	// Strategy Configuration:
	// Available Operators:
	// - condition.EqualTo
	// - condition.IsAbove
	// - condition.IsBelow
	// - condition.CrossesAbove
	// - condition.CrossesBelow
	// - condition.EqualOrAbove
	// - condition.EqualOrBelow
	cfg := strategy.Config{
		StartDate: "01-08-2025",
		EndDate:   "28-08-2025",
		Indicator: "awesomeOscillator", // Select which indicator from pipeConfigs to run strategy on

		Entry: condition.Rule{
			Operator: condition.CrossesAbove,
			Value:    0,
		},
		Exit: condition.Rule{
			Operator: condition.CrossesBelow,
			Value:    0,
		},
	}

	// Execute Backtest Strategy
	strategy.Run(candles, pl, cfg)
}
