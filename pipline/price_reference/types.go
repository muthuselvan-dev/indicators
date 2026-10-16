package price_reference

import (
	"indicators/core"
	"time"
)

// ScalarResult records the latest valid value and the preceding ready value.
type ScalarResult struct {
	IsReady   bool
	Current   float64
	Previous  float64
	Timestamp time.Time
}

type CurrentCandleResult struct {
	IsReady   bool
	Current   core.MarketCandle
	Previous  core.MarketCandle
	Timestamp time.Time
}

type EntryCandleResult struct {
	IsReady   bool
	Current   core.MarketCandle
	Previous  core.MarketCandle
	Timestamp time.Time
}

type LastNCandlesResult struct {
	IsReady   bool
	Current   []core.MarketCandle
	Previous  []core.MarketCandle
	Timestamp time.Time
}

type PreviousCandleResult struct {
	IsReady   bool
	Current   core.MarketCandle
	Previous  core.MarketCandle
	Timestamp time.Time
}

type PreviousDayCandleResult struct {
	IsReady   bool
	Current   core.MarketCandle
	Previous  core.MarketCandle
	Timestamp time.Time
}

type TodayCandleResult struct {
	IsReady   bool
	Current   float64
	Previous  float64
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Timestamp time.Time
}

type PreviousNthCandleSnapshot struct {
	Open         float64
	High         float64
	Low          float64
	Close        float64
	Volume       float64
	OpenInterest float64
	Timestamp    time.Time
}

type PreviousNthCandleResult struct {
	IsReady   bool
	Current   PreviousNthCandleSnapshot
	Previous  PreviousNthCandleSnapshot
	Timestamp time.Time
}
