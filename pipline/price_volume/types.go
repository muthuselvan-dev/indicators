package price_volume

import "time"

// ScalarResult records the latest valid value and the preceding ready value.
type ScalarResult struct {
	IsReady   bool
	Current   float64
	Previous  float64
	Timestamp time.Time
}

type CentralPivotRangeResult struct {
	IsReady   bool
	Pivot     float64
	Top       float64
	Bottom    float64
	Timestamp time.Time
}

type OpeningRangeBreakoutResult struct {
	IsReady   bool
	Current   float64
	Previous  float64
	High      float64
	Low       float64
	Breakout  int
	Timestamp time.Time
}

type PivotPointsResult struct {
	IsReady     bool
	Pivot       float64
	Resistance1 float64
	Support1    float64
	Resistance2 float64
	Support2    float64
	Resistance3 float64
	Support3    float64
	Resistance4 float64
	Support4    float64
	Resistance5 float64
	Support5    float64
	Timestamp   time.Time
}

type RangeBreakoutResult struct {
	IsReady   bool
	Current   float64
	Previous  float64
	High      float64
	Low       float64
	Breakout  int
	Timestamp time.Time
}
