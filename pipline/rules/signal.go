package rules

import (
	"fmt"
	"indicators/core"
	"time"
)

// describe: Signal
//
// pipeConfigs: [
//   {type: 'signal', name: 'signal', config: {}},
// ]
//
// Available result: Current, Previous, IsReady, Timestamp.
// Entry Condition: Requires verified QuantMan-specific inputs and rules.
// Exit Condition: Requires verified QuantMan-specific inputs and rules.
//
// Original Formula: No universal candle-only mathematical formula exists
// for this QuantMan strategy/integration node. It needs external signals,
// basket legs, position state, or user-configured expressions. A guessed
// calculation would generate incorrect orders; therefore it fails closed.
// Source: QuantMan-specific configuration not independently verified.
// Warm-up: Not applicable until the required input contract is supplied.

type SignalConfig struct{ CandleInterval string }
type Signal struct {
	id     string
	result ScalarResult
}

func NewSignal(id string, cfg SignalConfig) (*Signal, error) {
	if id == "" {
		return nil, fmt.Errorf("Signal: name cannot be empty")
	}
	return nil, fmt.Errorf("Signal: requires verified external/strategy input contract; not implementable from MarketCandle alone")
}
func (r *Signal) Update(c core.MarketCandle) any { r.ingest(c); return r.result }
func (r *Signal) Result() any                    { return r.result }
func (r *Signal) ingest(c core.MarketCandle) {
	if c != nil {
		r.result = ScalarResult{Timestamp: c.GetTime()}
	}
}

var _ core.Indicator = (*Signal)(nil)
var _ time.Time
