package rules

import (
	"fmt"
	"indicators/core"
	"time"
)

// describe: Manual Trigger
//
// pipeConfigs: [
//   {type: 'manualTrigger', name: 'manualTrigger', config: {}},
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

type ManualTriggerConfig struct{ CandleInterval string }
type ManualTrigger struct {
	id     string
	result ScalarResult
}

func NewManualTrigger(id string, cfg ManualTriggerConfig) (*ManualTrigger, error) {
	if id == "" {
		return nil, fmt.Errorf("Manual Trigger: name cannot be empty")
	}
	return nil, fmt.Errorf("Manual Trigger: requires verified external/strategy input contract; not implementable from MarketCandle alone")
}
func (r *ManualTrigger) Update(c core.MarketCandle) any { r.ingest(c); return r.result }
func (r *ManualTrigger) Result() any                    { return r.result }
func (r *ManualTrigger) ingest(c core.MarketCandle) {
	if c != nil {
		r.result = ScalarResult{Timestamp: c.GetTime()}
	}
}

var _ core.Indicator = (*ManualTrigger)(nil)
var _ time.Time
