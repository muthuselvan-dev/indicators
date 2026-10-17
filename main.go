package main

import (
	"fmt"
	"log"

	"indicators/aggregate"
	"indicators/datasource"
)

const printDate = "2025-08-28" // "" = all days

func main() {

	// Standard Candle
	// aggregateConfig := map[string]any{
	// 	"type":           "candle",
	// 	"candleInterval": "1 minute",
	// }

	// Heikin Ashi
	// aggregateConfig := map[string]any{
	// 	"type":           "heikinashi",
	// 	"candleInterval": "1 h",
	// }

	// Volume Candle
	aggregateConfig := map[string]any{
		"type":           "volumeCandle",
		"candleInterval": "1 h",
		"volumeInterval": "1 h",
	}

	engine, err := aggregate.NewAggregator(aggregateConfig)
	if err != nil {
		log.Fatal(err)
	}

	candles, err := datasource.ReadCSV("output.csv")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Date Time          Open         High         Low          Close        Volume")
	fmt.Println("--------------------------------------------------------------------------------")

	count := 0

	for _, candle := range candles {
		t := candle.GetTime()

		// NSE market hours: 09:15 - 15:29
		if t.Hour() < 9 || (t.Hour() == 9 && t.Minute() < 15) {
			continue
		}

		if t.Hour() > 15 || (t.Hour() == 15 && t.Minute() >= 30) {
			continue
		}

		// Aggregate candle using selected chart type
		result, ready := engine.Aggregate(candle)
		if !ready || result == nil {
			continue
		}

		if printDate != "" &&
			result.GetTime().Format("2006-01-02") != printDate {
			continue
		}

		fmt.Printf(
			"%s  %12.2f %12.2f %12.2f %12.2f %12.0f\n",
			result.GetTime().Format("02-01-2006 15:04"),
			result.GetOpen(),
			result.GetHigh(),
			result.GetLow(),
			result.GetClose(),
			result.GetVolume(),
		)

		count++
	}

	fmt.Printf("\nTotal chart candles: %d\n", count)
}
