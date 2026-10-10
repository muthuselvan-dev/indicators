package price_reference

import (
	"fmt"
	"indicators/core"
)

// describe: Signal Candle
//
// pipeConfigs: [
//   {type: 'signalCandle', name: 'signalCandle', config: {}},
// ]
//
// Available result: Current, Previous, IsReady, Timestamp.
// Entry Condition: Depends on the strategy's explicit signal/anchor state.
// Exit Condition: Depends on the strategy's explicit signal/anchor state.
// Original Formula: Signal candle is strategy-defined; no automatic signal inference from OHLCV.
// Source: QuantMan strategy-specific behavior is not publicly verified.
// Warm-up: Requires external strategy context; no fabricated candle signal.

type SignalCandleConfig struct{ CandleInterval string }
type SignalCandle struct {
	id     string
	result ScalarResult
}

func NewSignalCandle(id string, cfg SignalCandleConfig) (*SignalCandle, error) {
	if id == "" {
		return nil, fmt.Errorf("Signal Candle: empty name")
	}
	return nil, fmt.Errorf("Signal Candle: requires strategy context not provided by core.MarketCandle")
}
func (r *SignalCandle) Update(c core.MarketCandle) any { r.ingest(c); return r.result }
func (r *SignalCandle) Result() any                    { return r.result }
func (r *SignalCandle) ingest(c core.MarketCandle) {
	if c != nil {
		r.result.Timestamp = c.GetTime()
	}
}

var _ core.Indicator = (*SignalCandle)(nil)
