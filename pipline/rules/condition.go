package rules

import (
	"fmt"
	"indicators/core"
	"time"
)

// describe: Condition
//
// pipeConfigs: [
//   {type: 'condition', name: 'condition', config: {}},
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

type ConditionConfig struct{ CandleInterval string }
type Condition struct {
	id     string
	result ScalarResult
}

func NewCondition(id string, cfg ConditionConfig) (*Condition, error) {
	if id == "" {
		return nil, fmt.Errorf("Condition: name cannot be empty")
	}
	return nil, fmt.Errorf("Condition: requires verified external/strategy input contract; not implementable from MarketCandle alone")
}
func (r *Condition) Update(c core.MarketCandle) any { r.ingest(c); return r.result }
func (r *Condition) Result() any                    { return r.result }
func (r *Condition) ingest(c core.MarketCandle) {
	if c != nil {
		r.result = ScalarResult{Timestamp: c.GetTime()}
	}
}

var _ core.Indicator = (*Condition)(nil)
var _ time.Time
