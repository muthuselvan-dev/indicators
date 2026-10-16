package aggregate

import "indicators/core"

type ChartProcessor interface {
	Update(core.MarketCandle) (core.MarketCandle, bool)
}
