package core

type Indicator interface {
	Update(candle MarketCandle) any
	Result() any
}
