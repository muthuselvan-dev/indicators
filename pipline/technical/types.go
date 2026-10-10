package technical

import "time"

// ScalarResult is the most recent calculated indicator value.
// Previous is the preceding ready value; IsReady is false during warm-up.
type ScalarResult struct {
	IsReady   bool
	Current   float64
	Previous  float64
	Timestamp time.Time
}
