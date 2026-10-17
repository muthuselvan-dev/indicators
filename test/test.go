package main

import (
	"errors"
	"fmt"
	"time"

	"indicators/datafeed"
)

var errTestCompleted = errors.New("test completed")

func main() {
	df, err := datafeed.NewDataFeed()

	if err != nil {
		fmt.Println("Error creating DataFeed:", err)
		return
	}

	ist := time.FixedZone("IST", 5*3600+30*60)

	// Token -> Ticker Name lookup
	tickerMap := make(map[int]string, len(df.Tickers))

	for _, ticker := range df.Tickers {
		tickerMap[ticker.Token] = ticker.TradingSymbol
	}

	count := 0

	err = df.FeedData(func(candles []datafeed.Candle) error {

		if len(candles) == 0 {
			return nil
		}

		count++

		currentTime := candles[0].Timestamp.In(ist).Format("15:04")

		fmt.Printf("\n========== CANDLE BATCH %d ==========\n", count)

		for _, c := range candles {
			fmt.Printf(
				"Time: %s | Ticker: %s | Token: %d | O: %.2f H: %.2f L: %.2f C: %.2f | V: %d | OI: %d\n",
				c.Timestamp.In(ist).Format("2006-01-02 15:04:05"),
				tickerMap[c.Token],
				c.Token,
				c.Open,
				c.High,
				c.Low,
				c.Close,
				c.Volume,
				c.OpenInterest,
			)
		}

		// Subscribe at 09:20 IST
		if currentTime == "09:18" {
			if err := df.Subscribe(3); err != nil {
				return err
			}

			fmt.Println("SUBSCRIBED:", tickerMap[3], "| Token: 3")
		}

		// Unsubscribe at 09:25 IST
		if currentTime == "09:25" {
			if err := df.Unsubscribe(3); err != nil {
				return err
			}

			fmt.Println("UNSUBSCRIBED:", tickerMap[3], "| Token: 3")
		}

		if count >= 80 {
			return errTestCompleted
		}

		time.Sleep(10 * time.Second)

		return nil
	})

	if err != nil && !errors.Is(err, errTestCompleted) {
		fmt.Println("DataFeed Error:", err)
		return
	}

	fmt.Println("\nTest Finished")
}
