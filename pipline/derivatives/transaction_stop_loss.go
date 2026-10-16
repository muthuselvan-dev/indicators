package derivatives

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"indicators/core"
)

// describe: Transaction StopLoss
//
// Transaction StopLoss is a transaction-level risk management rule.
// It monitors an active transaction and exits when its configured
// stop-loss level is reached.
//
// Supported Configuration:
//
// pipeConfigs: [
//   {
//     type: 'transactionStopLoss',
//     name: 'transactionStopLoss',
//     config: {
//       caseIndex: '0',
//       stopLoss: {
//         type: 'Percent',
//         value: 50,
//         trail: {
//           type: 'Linear',
//           adjustValue: 5,
//           trailingValue: 5
//         }
//       }
//     }
//   }
// ]
//
// Config:
//
//   caseIndex:
//     Transaction/case identifier supplied by the strategy.
//
//   stopLoss.type:
//     Percent.
//
//   stopLoss.value:
//     Initial stop-loss percentage.
//
//   trail.type:
//     Linear.
//
//   trail.trailingValue:
//     Favorable price movement percentage needed for one step.
//
//   trail.adjustValue:
//     Percentage points of entry price used to adjust SL per step.
//
// Available Result:
//
//   Current
//   Previous
//   IsReady
//   EntryPrice
//   CurrentPrice
//   StopLossPrice
//   ProfitPercent
//   TrailingSteps
//   Timestamp
//
// Entry Condition:
//
//   Not an independent entry indicator.
//   The transaction must already be active.
//
// Exit Condition:
//
//   Transaction StopLoss Is True:
//     Current == true
//
//   Transaction StopLoss Becomes True:
//     !Previous && Current
//
// Formula (reference price-based model):
//
//   Initial Buy SL = EntryPrice * (1 - SLPercent / 100)
//
//   Initial Sell SL = EntryPrice * (1 + SLPercent / 100)
//
//   Favorable Move % (Buy) =
//     (HighestPrice - EntryPrice) / EntryPrice * 100
//
//   Favorable Move % (Sell) =
//     (EntryPrice - LowestPrice) / EntryPrice * 100
//
//   Steps = floor(FavorableMove% / TrailingValue)
//
//   Adjustment = Steps * AdjustValue / 100 * EntryPrice
//
//   Buy Trailing SL = InitialBuySL + Adjustment
//
//   Sell Trailing SL = InitialSellSL - Adjustment
//
//   StopLoss Hit (Buy) = CurrentPrice <= StopLossPrice
//
//   StopLoss Hit (Sell) = CurrentPrice >= StopLossPrice
//
// Example:
//
//   Entry Price = 100
//   StopLoss = 50%
//   TrailingValue = 5%
//   AdjustValue = 5%
//
//   Price 100 -> SL 50
//   Price 105 -> SL 55
//   Price 110 -> SL 60
//   Price 120 -> SL 70
//
// Warmup:
//
//   No historical candle warmup.
//   OpenPosition() initializes the transaction.
//   UpdatePrice() evaluates the active transaction.
//
// Limitations:
//
//   This is a price-based reference implementation.
//   QuantMan transaction-level percentage semantics and exact
//   trailing calculation have not been verified.
//   No transaction aggregation, slippage, or intrabar High/Low
//   execution simulation is included.
//   Update(candle) fails closed without transaction price context.

func NewTransactionStopLoss(id string, cfg map[string]any) (*TransactionStopLoss, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("TransactionStopLoss: name cannot be empty")
	}

	caseIndex, ok := cfg["caseIndex"].(string)
	if !ok {
		return nil, fmt.Errorf("TransactionStopLoss: caseIndex must be a string")
	}

	index, err := strconv.Atoi(caseIndex)
	if err != nil || index < 0 {
		return nil, fmt.Errorf("TransactionStopLoss: invalid caseIndex %q", caseIndex)
	}

	sl, ok := cfg["stopLoss"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("TransactionStopLoss: stopLoss config required")
	}

	slType, _ := sl["type"].(string)
	if !strings.EqualFold(slType, "Percent") {
		return nil, fmt.Errorf("TransactionStopLoss: unsupported stopLoss type %q", slType)
	}

	value, err := transactionSLNumber(sl["value"])
	if err != nil || value <= 0 {
		return nil, fmt.Errorf("TransactionStopLoss: invalid stopLoss value: %v", sl["value"])
	}

	trail, ok := sl["trail"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("TransactionStopLoss: trail config required")
	}

	trailType, _ := trail["type"].(string)
	if !strings.EqualFold(trailType, "Linear") {
		return nil, fmt.Errorf("TransactionStopLoss: unsupported trail type %q", trailType)
	}

	adjust, err := transactionSLNumber(trail["adjustValue"])
	if err != nil || adjust <= 0 {
		return nil, fmt.Errorf("TransactionStopLoss: invalid adjustValue")
	}

	trailing, err := transactionSLNumber(trail["trailingValue"])
	if err != nil || trailing <= 0 {
		return nil, fmt.Errorf("TransactionStopLoss: invalid trailingValue")
	}

	return &TransactionStopLoss{
		id: id,
		config: TransactionStopLossConfig{
			CaseIndex: caseIndex,
			StopLoss: TransactionStopLossValue{
				Type:  slType,
				Value: value,
				Trail: TransactionStopLossTrail{
					Type:          trailType,
					AdjustValue:   adjust,
					TrailingValue: trailing,
				},
			},
		},
	}, nil
}

// OpenPosition initializes a new transaction.
func (r *TransactionStopLoss) OpenPosition(entry float64, side string) error {
	if r == nil {
		return fmt.Errorf("TransactionStopLoss: nil receiver")
	}

	if r.active {
		return fmt.Errorf("TransactionStopLoss: transaction already active")
	}

	if entry <= 0 || math.IsNaN(entry) || math.IsInf(entry, 0) {
		return fmt.Errorf("TransactionStopLoss: invalid entry price")
	}

	side = strings.ToLower(strings.TrimSpace(side))
	if side != "buy" && side != "sell" {
		return fmt.Errorf("TransactionStopLoss: side must be buy or sell")
	}

	r.active = true
	r.side = side
	r.entry = entry
	r.bestPrice = entry

	pct := r.config.StopLoss.Value / 100

	if side == "buy" {
		r.stopPrice = entry * (1 - pct)
	} else {
		r.stopPrice = entry * (1 + pct)
	}

	r.result = TransactionStopLossResult{
		IsReady:       true,
		EntryPrice:    entry,
		CurrentPrice:  entry,
		StopLossPrice: r.stopPrice,
	}

	return nil
}

// UpdatePrice evaluates an active transaction.
func (r *TransactionStopLoss) UpdatePrice(price float64, timestamp time.Time) TransactionStopLossResult {
	if r == nil {
		return TransactionStopLossResult{}
	}

	if !r.active || price <= 0 || math.IsNaN(price) || math.IsInf(price, 0) {
		r.result.IsReady = false
		return r.result
	}

	previous := r.result.Current

	// Check existing stop-loss before trailing adjustment.
	hit := false

	if r.side == "buy" {
		hit = price <= r.stopPrice
	} else {
		hit = price >= r.stopPrice
	}

	if !hit {
		if r.side == "buy" && price > r.bestPrice {
			r.bestPrice = price
		}

		if r.side == "sell" && price < r.bestPrice {
			r.bestPrice = price
		}

		favorablePercent := 0.0

		if r.side == "buy" {
			favorablePercent = (r.bestPrice - r.entry) / r.entry * 100
		} else {
			favorablePercent = (r.entry - r.bestPrice) / r.entry * 100
		}

		trail := r.config.StopLoss.Trail

		steps := int(math.Floor(
			favorablePercent / trail.TrailingValue,
		))

		adjustment := float64(steps) *
			trail.AdjustValue / 100 * r.entry

		initialPercent := r.config.StopLoss.Value / 100

		if r.side == "buy" {
			newSL := r.entry*(1-initialPercent) + adjustment
			if newSL > r.stopPrice {
				r.stopPrice = newSL
			}
		} else {
			newSL := r.entry*(1+initialPercent) - adjustment
			if newSL < r.stopPrice {
				r.stopPrice = newSL
			}
		}
	}

	profitPercent := (price - r.entry) / r.entry * 100
	if r.side == "sell" {
		profitPercent = -profitPercent
	}

	favorablePercent := 0.0

	if r.side == "buy" {
		favorablePercent = (r.bestPrice - r.entry) / r.entry * 100
	} else {
		favorablePercent = (r.entry - r.bestPrice) / r.entry * 100
	}

	steps := int(math.Floor(
		favorablePercent / r.config.StopLoss.Trail.TrailingValue,
	))

	r.result = TransactionStopLossResult{
		IsReady:       true,
		Current:       hit,
		Previous:      previous,
		StopLossPrice: r.stopPrice,
		CurrentPrice:  price,
		EntryPrice:    r.entry,
		ProfitPercent: profitPercent,
		TrailingSteps: steps,
		Timestamp:     timestamp,
	}

	if hit {
		r.active = false
	}

	return r.result
}

// ClosePosition resets the transaction state.
func (r *TransactionStopLoss) ClosePosition() {
	if r == nil {
		return
	}

	r.active = false
	r.side = ""
	r.entry = 0
	r.bestPrice = 0
	r.stopPrice = 0
	r.result = TransactionStopLossResult{}
}

// Update implements core.Indicator.
// MarketCandle alone does not identify an active transaction.
func (r *TransactionStopLoss) Update(c core.MarketCandle) any {
	if r == nil {
		return TransactionStopLossResult{}
	}

	r.result.IsReady = false
	return r.result
}

func (r *TransactionStopLoss) Result() any {
	if r == nil {
		return TransactionStopLossResult{}
	}

	return r.result
}

func transactionSLNumber(v any) (float64, error) {
	var value float64

	switch n := v.(type) {
	case float64:
		value = n
	case float32:
		value = float64(n)
	case int:
		value = float64(n)
	case int64:
		value = float64(n)
	default:
		return 0, fmt.Errorf("invalid numeric value: %v", v)
	}

	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, fmt.Errorf("invalid non-finite numeric value")
	}

	return value, nil
}

var _ core.Indicator = (*TransactionStopLoss)(nil)
