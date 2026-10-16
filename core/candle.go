package core

import "time"

// Candle is the aggregated candle.
// It implements MarketCandle.

type MarketCandle interface {
	GetTime() time.Time
	GetOpen() float64
	GetHigh() float64
	GetLow() float64
	GetClose() float64
	GetVolume() float64
	GetOpenInterest() float64
}

type Candle struct {
	Time         time.Time
	Open         float64
	High         float64
	Low          float64
	Close        float64
	Volume       float64
	OpenInterest float64
}

func (c Candle) GetTime() time.Time       { return c.Time }
func (c Candle) GetOpen() float64         { return c.Open }
func (c Candle) GetHigh() float64         { return c.High }
func (c Candle) GetLow() float64          { return c.Low }
func (c Candle) GetClose() float64        { return c.Close }
func (c Candle) GetVolume() float64       { return c.Volume }
func (c Candle) GetOpenInterest() float64 { return c.OpenInterest }
