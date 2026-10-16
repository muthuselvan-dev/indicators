package derivatives

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: Ratio
//
// pipeConfigs: [
//   {
//     type: 'ratio',
//     name: 'ratio',
//     config: {}
//   }
// ]
//
// chartConfig and valuePaths are not used.
//
// Available result:
//   - Current
//   - Previous
//   - IsReady
//   - Timestamp
//
// Note:
// QuantMan Ratio calculation formula is not yet verified.
// Do not use an assumed formula for backtesting.

type Ratio struct {
	id string

	lastTimestamp time.Time

	result RatioResult
}

func NewRatio(id string, cfg map[string]any) (*Ratio, error) {
	if id == "" {
		return nil, fmt.Errorf("Ratio name cannot be empty")
	}

	return &Ratio{
		id: id,
	}, nil
}

func (r *Ratio) Update(candle core.MarketCandle) any {
	r.ingest(candle)
	return r.result
}

func (r *Ratio) Result() any {
	return r.result
}

func (r *Ratio) ingest(candle core.MarketCandle) {
	if candle == nil {
		r.result = RatioResult{}
		return
	}

	closePrice := candle.GetClose()
	timestamp := candle.GetTime()

	// Reject invalid candle data.
	if timestamp.IsZero() ||
		math.IsNaN(closePrice) ||
		math.IsInf(closePrice, 0) ||
		closePrice <= 0 ||
		(!r.lastTimestamp.IsZero() &&
			!timestamp.After(r.lastTimestamp)) {

		r.result = RatioResult{
			IsReady:   false,
			Timestamp: timestamp,
		}
		return
	}

	r.lastTimestamp = timestamp

	// Formula pending verification.
	// Never publish an unverified Ratio as ready.
	r.result = RatioResult{
		IsReady:   false,
		Timestamp: timestamp,
	}
}
