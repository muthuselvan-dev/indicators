package price_reference

import "time"

// ScalarResult records the latest valid value and the preceding ready value.
type ScalarResult struct {
	IsReady   bool
	Current   float64
	Previous  float64
	Timestamp time.Time
}

type PreviousCandleResult struct {
	IsReady   bool
	Current   PreviousCandleSnapshot
	Previous  PreviousCandleSnapshot
	Timestamp time.Time
}