package price_volume

import "time"

// ScalarResult records the latest valid value and the preceding ready value.
type ScalarResult struct {
	IsReady   bool
	Current   float64
	Previous  float64
	Timestamp time.Time
}
