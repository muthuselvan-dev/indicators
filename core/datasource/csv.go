package datasource

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

type Candle struct {
	Ticker       string
	Timestamp    time.Time
	Open         float64
	High         float64
	Low          float64
	Close        float64
	Volume       float64
	OpenInterest float64
}

func (c Candle) GetTime() time.Time       { return c.Timestamp }
func (c Candle) GetOpen() float64         { return c.Open }
func (c Candle) GetHigh() float64         { return c.High }
func (c Candle) GetLow() float64          { return c.Low }
func (c Candle) GetClose() float64        { return c.Close }
func (c Candle) GetVolume() float64       { return c.Volume }
func (c Candle) GetOpenInterest() float64 { return c.OpenInterest }

func ReadCSV(path string) ([]Candle, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)

	if _, err := reader.Read(); err != nil {
		return nil, err
	}

	var candles []Candle

	for {
		record, err := reader.Read()

		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if len(record) < 8 {
			return nil, fmt.Errorf("invalid CSV row: %v", record)
		}

		timestamp, err := time.Parse(
			"02-01-2006 15:04:05",
			strings.TrimSpace(record[1]),
		)
		if err != nil {
			return nil, err
		}

		values := make([]float64, 6)

		for i, col := range []int{2, 3, 4, 5, 6, 7} {
			values[i], err = strconv.ParseFloat(
				strings.TrimSpace(record[col]), 64,
			)
			if err != nil {
				return nil, err
			}
		}

		candles = append(candles, Candle{
			Ticker:       strings.TrimSpace(record[0]),
			Timestamp:    timestamp,
			Open:         values[0],
			High:         values[1],
			Low:          values[2],
			Close:        values[3],
			Volume:       values[4],
			OpenInterest: values[5],
		})
	}

	return candles, nil
}
