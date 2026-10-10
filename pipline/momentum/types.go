package momentum

import "time"

type RSIBaseResult struct {
	IsReady   bool
	Current   float64
	Previous  float64
	Timestamp time.Time
}

type RSIResult struct {
	RSIBaseResult
}

type CCIResult struct {
	IsReady   bool
	Current   float64
	Previous  float64
	Timestamp time.Time
}

type MFIResult struct {
	IsReady   bool
	Current   float64
	Previous  float64
	Timestamp time.Time
}

type StochasticOscillatorResult struct {
	IsReady   bool
	Current   float64
	Previous  float64
	K         float64
	D         float64
	PreviousK float64
	PreviousD float64
	Timestamp time.Time
}

type StochasticRSIResult struct {
	IsReady   bool
	Current   float64
	Previous  float64
	K         float64
	D         float64
	PreviousK float64
	PreviousD float64
	Timestamp time.Time
}

type WilliamsPercentRangeResult struct {
	IsReady   bool
	Current   float64
	Previous  float64
	Timestamp time.Time
}

type ROCResult struct {
	IsReady   bool
	Current   float64
	Previous  float64
	Timestamp time.Time
}

type MACDResult struct {
	IsReady           bool
	Current           float64
	Previous          float64
	Signal            float64
	PreviousSignal    float64
	Histogram         float64
	PreviousHistogram float64
	Timestamp         time.Time
}

type AOResult struct {
	IsReady   bool
	Current   float64
	Previous  float64
	Timestamp time.Time
}
