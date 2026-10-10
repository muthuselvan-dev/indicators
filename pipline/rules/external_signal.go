package rules

import (
	"fmt"
	"indicators/core"
	"time"
)

// describe: External Signal
//
// pipeConfigs: [
//   {type: 'externalSignal', name: 'externalSignal', config: {}},
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

type ExternalSignalConfig struct{ CandleInterval string }
type ExternalSignal struct {
	id     string
	result ScalarResult
}

func NewExternalSignal(id string, cfg ExternalSignalConfig) (*ExternalSignal, error) {
	if id == "" {
		return nil, fmt.Errorf("External Signal: name cannot be empty")
	}
	return nil, fmt.Errorf("External Signal: requires verified external/strategy input contract; not implementable from MarketCandle alone")
}
func (r *ExternalSignal) Update(c core.MarketCandle) any { r.ingest(c); return r.result }
func (r *ExternalSignal) Result() any                    { return r.result }
func (r *ExternalSignal) ingest(c core.MarketCandle) {
	if c != nil {
		r.result = ScalarResult{Timestamp: c.GetTime()}
	}
}

var _ core.Indicator = (*ExternalSignal)(nil)
var _ time.Time
