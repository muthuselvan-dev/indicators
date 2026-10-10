package price_reference

import (
	"fmt"
	"indicators/core"
)

// describe: Entry Candle
//
// pipeConfigs: [
//   {type: 'entryCandle', name: 'entryCandle', config: {}},
// ]
//
// Available result: Current, Previous, IsReady, Timestamp.
// Entry Condition: Depends on the strategy's explicit signal/anchor state.
// Exit Condition: Depends on the strategy's explicit signal/anchor state.
// Original Formula: The entry candle must be supplied by the strategy execution state; candle stream alone cannot identify it.
// Source: QuantMan strategy-specific behavior is not publicly verified.
// Warm-up: Requires external strategy context; no fabricated candle signal.

type EntryCandleConfig struct{ CandleInterval string }
type EntryCandle struct {
	id     string
	result ScalarResult
}

func NewEntryCandle(id string, cfg EntryCandleConfig) (*EntryCandle, error) {
	if id == "" {
		return nil, fmt.Errorf("Entry Candle: empty name")
	}
	return nil, fmt.Errorf("Entry Candle: requires strategy context not provided by core.MarketCandle")
}
func (r *EntryCandle) Update(c core.MarketCandle) any { r.ingest(c); return r.result }
func (r *EntryCandle) Result() any                    { return r.result }
func (r *EntryCandle) ingest(c core.MarketCandle) {
	if c != nil {
		r.result.Timestamp = c.GetTime()
	}
}

var _ core.Indicator = (*EntryCandle)(nil)
