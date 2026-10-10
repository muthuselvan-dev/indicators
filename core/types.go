package core

import "time"

type MarketCandle interface {
	GetTime() time.Time
	GetOpen() float64
	GetHigh() float64
	GetLow() float64
	GetClose() float64
	GetVolume() float64
	GetOpenInterest() float64
}

type Indicator interface {
	Update(candle MarketCandle) any
	Result() any
}
