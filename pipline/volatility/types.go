package volatility

import (
	"time"
)

// BaseResult stores the most recent and previous ready indicator values.
type BaseResult struct {
	IsReady   bool
	Current   float64
	Previous  float64
	Timestamp time.Time
}

type AverageTrueRangeResult struct{ BaseResult }

type BollingerBandResult struct {
	BaseResult
	Middle            float64
	Upper             float64
	Lower             float64
	StandardDeviation float64
}
type DonchianChannelResult struct {
	BaseResult
	Upper  float64
	Lower  float64
	Middle float64
}

type ScalarResult = BaseResult
