package datafeed

import (
	"database/sql"
	"fmt"
	"runtime"
	"time"

	_ "github.com/lib/pq"
)

type Ticker struct {
	Token          int
	TradingSymbol  string
	Symbol         string
	InstrumentType string
	ExpiryTag      string
	Exchange       string
	StrikePrice    float64
	LotSize        int
	StrikeStep     float64
}

type Candle struct {
	Token        int
	Timestamp    time.Time
	Open         float64
	High         float64
	Low          float64
	Close        float64
	Volume       int64
	OpenInterest int64
}

func (df *DataFeed) loadDatabase() error {
	var before, after runtime.MemStats

	runtime.GC()
	runtime.ReadMemStats(&before)

	fmt.Printf("Memory Before: %.2f MB\n", float64(before.Alloc)/1024/1024)

	db, err := sql.Open(
		"postgres",
		"host=localhost port=5432 user=postgres password=YOUR_PASSWORD dbname=market_data_db sslmode=disable",
	)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return err
	}

	tickerRows, err := db.Query(`
		SELECT token, trading_symbol, symbol, instrument_type,
		       expiry_tag, exchange, strike_price,
		       COALESCE(lot_size, 0), COALESCE(strike_step, 0)
		FROM ticker_map
		WHERE symbol = 'BANKNIFTY'
		ORDER BY token
	`)
	if err != nil {
		return err
	}

	for tickerRows.Next() {
		var t Ticker

		if err := tickerRows.Scan(
			&t.Token, &t.TradingSymbol, &t.Symbol,
			&t.InstrumentType, &t.ExpiryTag, &t.Exchange,
			&t.StrikePrice, &t.LotSize, &t.StrikeStep,
		); err != nil {
			tickerRows.Close()
			return err
		}

		df.Tickers = append(df.Tickers, t)
	}

	if err := tickerRows.Err(); err != nil {
		tickerRows.Close()
		return err
	}
	tickerRows.Close()

	candleRows, err := db.Query(`
		SELECT c.token, c.timestamp, c.open_price, c.high_price,
		       c.low_price, c.close_price, c.volume, c.open_interest
		FROM contract_data c
		JOIN ticker_map t ON t.token = c.token
		WHERE t.symbol = 'BANKNIFTY'
		ORDER BY c.token, c.timestamp
	`)
	if err != nil {
		return err
	}

	for candleRows.Next() {
		var c Candle

		if err := candleRows.Scan(
			&c.Token, &c.Timestamp, &c.Open, &c.High,
			&c.Low, &c.Close, &c.Volume, &c.OpenInterest,
		); err != nil {
			candleRows.Close()
			return err
		}

		df.Candles = append(df.Candles, c)
	}

	if err := candleRows.Err(); err != nil {
		candleRows.Close()
		return err
	}
	candleRows.Close()

	runtime.GC()
	runtime.ReadMemStats(&after)

	fmt.Printf("Memory After : %.2f MB\n", float64(after.Alloc)/1024/1024)
	fmt.Printf("Memory Used  : %.2f MB\n", float64(after.Alloc-before.Alloc)/1024/1024)
	fmt.Printf("Total Tickers: %d\n", len(df.Tickers))
	fmt.Printf("Total Candles: %d\n", len(df.Candles))

	return nil
}
