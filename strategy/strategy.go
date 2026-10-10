package strategy

import (
	"fmt"
	"log"
	"strings"
	"time"

	"indicators/condition"
	"indicators/core/datasource"
	"indicators/pipline"
)

type Config struct {
	StartDate string         // "01-08-2025"
	EndDate   string         // "28-08-2025"
	Indicator string         // Main indicator name from pipeConfigs (e.g. "rsi", "commodityChannelIndex")
	Entry     condition.Rule // Entry rule (Operator & Value)
	Exit      condition.Rule // Exit rule (Operator & Value)
}

type Trade struct {
	Instrument string
	EntryTime  string
	EntryPrice float64
	ExitTime   string
	ExitPrice  float64
}

// Run executes a backtest strategy against the pipeline.
func Run(candles []datasource.Candle, pl *pipline.Pipeline, cfg Config) []Trade {
	if pl == nil {
		log.Fatal("pipeline cannot be nil")
	}

	startDate, err := time.Parse("02-01-2006", cfg.StartDate)
	if err != nil {
		log.Fatalf("invalid start date format (expected DD-MM-YYYY): %v", err)
	}

	endDate, err := time.Parse("02-01-2006", cfg.EndDate)
	if err != nil {
		log.Fatalf("invalid end date format (expected DD-MM-YYYY): %v", err)
	}

	// Resolve entry and exit indicator names (default to cfg.Indicator if not explicitly set in the rule)
	entryIndicator := cfg.Entry.Indicator
	if entryIndicator == "" {
		entryIndicator = cfg.Indicator
	}

	exitIndicator := cfg.Exit.Indicator
	if exitIndicator == "" {
		exitIndicator = cfg.Indicator
	}

	fmt.Println("Instrument | Entry Time | Entry Price | Exit Time | Exit Price")

	var trades []Trade
	var entryPrice float64
	var entryTime, lastDate string
	inPosition, tradedToday := false, false

	lastConfirmedValues := make(map[string]float64)
	hasConfirmedValues := make(map[string]bool)

	for _, candle := range candles {
		tradeTime := candle.Timestamp.Format("15:04")

		// Filter out post-market / auction candles (after 15:29)
		if tradeTime > "15:29" || tradeTime < "09:15" {
			continue
		}

		candleDate := time.Date(candle.Timestamp.Year(), candle.Timestamp.Month(), candle.Timestamp.Day(), 0, 0, 0, 0, candle.Timestamp.Location())

		if candleDate.Before(startDate) {
			continue
		}

		if candleDate.After(endDate) {
			continue
		}

		date := candle.Timestamp.Format("2006-01-02")
		isFirstBarOfDay := false
		if date != lastDate {
			tradedToday = false
			lastDate = date
			isFirstBarOfDay = true
		}

		// Phase 1: Bar Open Event (Market Open 09:15)
		if !inPosition && !tradedToday && tradeTime == "09:15" {
			hasConf := hasConfirmedValues[entryIndicator]
			lastVal := lastConfirmedValues[entryIndicator]

			canEnter := false
			if isFirstBarOfDay && candleDate.Equal(startDate) && !hasConf {
				if !strings.Contains(strings.ToLower(string(cfg.Entry.Operator)), "cross") {
					canEnter = true
				}
			} else if cfg.Entry.CanEnterAtMarketOpen(hasConf, lastVal) {
				canEnter = true
			}

			if canEnter {
				inPosition = true
				tradedToday = true
				entryPrice = candle.Open
				// entryTime = candle.Timestamp.Format("02-01-2006") + " 09:15:14"
				entryTime = candle.Timestamp.Format("02-01-2006 15:04:05")
			}
		}

		// Phase 2: Bar Close Event (Update Indicators in Pipeline)
		results, err := pl.Process(candle)
		if err != nil {
			log.Fatal(err)
		}

		// Extract values dynamically for entry and exit indicators
		entryReady, entryCurr, entryPrev, entryOk := condition.ExtractValues(results[entryIndicator])
		exitReady, exitCurr, exitPrev, exitOk := condition.ExtractValues(results[exitIndicator])

		if entryOk && entryReady {
			lastConfirmedValues[entryIndicator] = entryCurr
			hasConfirmedValues[entryIndicator] = true
		}
		if exitOk && exitReady {
			lastConfirmedValues[exitIndicator] = exitCurr
			hasConfirmedValues[exitIndicator] = true
		}

		// Phase 3: Bar Close Strategy Evaluation
		closeTimeStr := candle.Timestamp.Add(1 * time.Second).Format("02-01-2006 15:04:00")
		squareOff := tradeTime >= "15:14"

		// Evaluate Dynamic Exit Condition
		if inPosition {
			shouldExit := squareOff
			if exitOk && exitReady && cfg.Exit.Evaluate(exitCurr, exitPrev) {
				shouldExit = true
			}

			if shouldExit {
				fmt.Printf("%s | %s | %.2f | %s | %.2f\n",
					candle.Ticker,
					entryTime,
					entryPrice,
					closeTimeStr,
					candle.Close,
				)

				trades = append(trades, Trade{
					Instrument: candle.Ticker,
					EntryTime:  entryTime,
					EntryPrice: entryPrice,
					ExitTime:   closeTimeStr,
					ExitPrice:  candle.Close,
				})

				inPosition = false
				continue
			}
		}

		// Evaluate Dynamic Intraday Entry Condition (if not entered at market open)
		if !inPosition && !tradedToday && tradeTime < "15:14" {
			if cfg.Entry.EvaluateEntry(isFirstBarOfDay, entryOk && entryReady, entryCurr, entryPrev) {
				inPosition = true
				tradedToday = true
				entryPrice = candle.Close
				entryTime = closeTimeStr
			}
		}
	}

	return trades
}
