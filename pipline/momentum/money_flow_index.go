package momentum

import (
	"fmt"
	"indicators/core"
	"math"
	"time"
)

// describe: Money Flow Index (MFI)
//
// pipeConfigs: [
//   {
//     type: 'moneyFlowIndex',
//     name: 'mfi',
//     config: {
//       period: 14,
//       candleInterval: '1 minutes',
//     },
//   },
// ]
//
// Available result:
//   - Money Flow Index
//
// Entry Condition:
//   MFI Crosses Above 20
//   Previous MFI <= 20 && Current MFI > 20
//
// Exit Condition:
//   MFI Crosses Below 80
//   Previous MFI >= 80 && Current MFI < 80
//
// Original Formula:
//   Typical Price = (High + Low + Close) / 3
//   Raw Money Flow = Typical Price * Volume
//
//   Positive Flow = Raw Money Flow if Typical Price increases
//   Negative Flow = Raw Money Flow if Typical Price decreases
//   Both flows = 0 if Typical Price is unchanged
//
//   Money Flow Ratio = Positive Flow Sum / Negative Flow Sum
//   MFI = 100 - (100 / (1 + Money Flow Ratio))
//
//   Both sums zero: MFI = 50
//   Negative sum zero: MFI = 100
//   Positive sum zero: MFI = 0
//
// BaseResult.Current represents current MFI.
// BaseResult.Previous represents previous MFI.
//
// First result requires period + 1 accepted candles.
// A valid crossover requires two calculated MFI values.
//
// Invalid or duplicate candles are rejected without
// changing the last calculated result.

type MFI struct {
	id string

	period         int
	candleInterval string

	previousTypicalPrice float64
	hasTypicalPrice      bool

	posFlows []float64
	negFlows []float64

	hasResult bool
	lastValue float64

	lastTimestamp time.Time

	result MFIResult
}

func NewMFI(id string, cfg map[string]any) (*MFI, error) {

	if id == "" {
		return nil, fmt.Errorf("MFI name cannot be empty")
	}

	return &MFI{
		id:             id,
		period:         cfg["period"].(int),
		candleInterval: cfg["candleInterval"].(string),
		posFlows:       make([]float64, 0, cfg["period"].(int)),
		negFlows:       make([]float64, 0, cfg["period"].(int)),
	}, nil
}

func (m *MFI) Update(candle core.MarketCandle) any {
	m.ingest(candle)
	return m.result
}

func (m *MFI) Result() any {
	return m.result
}

func (m *MFI) ingest(candle core.MarketCandle) {
	if candle == nil {
		return
	}

	high := candle.GetHigh()
	low := candle.GetLow()
	closePrice := candle.GetClose()
	volume := candle.GetVolume()
	timestamp := candle.GetTime()

	// Validate all source values before changing state.
	if timestamp.IsZero() ||
		math.IsNaN(high) ||
		math.IsNaN(low) ||
		math.IsNaN(closePrice) ||
		math.IsNaN(volume) ||
		math.IsInf(high, 0) ||
		math.IsInf(low, 0) ||
		math.IsInf(closePrice, 0) ||
		math.IsInf(volume, 0) ||
		high <= 0 ||
		low <= 0 ||
		high < low ||
		closePrice < low ||
		closePrice > high ||
		volume < 0 ||
		(!m.lastTimestamp.IsZero() &&
			!timestamp.After(m.lastTimestamp)) {
		return
	}

	// Weighted sum avoids intermediate overflow
	// from high + low + closePrice.
	typicalPrice := high/3 + low/3 + closePrice/3

	if math.IsNaN(typicalPrice) ||
		math.IsInf(typicalPrice, 0) ||
		typicalPrice <= 0 {
		return
	}

	// The first candle only establishes the reference.
	if !m.hasTypicalPrice {
		m.previousTypicalPrice = typicalPrice
		m.hasTypicalPrice = true
		m.lastTimestamp = timestamp

		m.result = MFIResult{
			IsReady:   false,
			Timestamp: timestamp,
		}
		return
	}

	rawMoneyFlow := typicalPrice * volume

	if math.IsNaN(rawMoneyFlow) ||
		math.IsInf(rawMoneyFlow, 0) {
		return
	}

	var posFlow, negFlow float64

	switch {
	case typicalPrice > m.previousTypicalPrice:
		posFlow = rawMoneyFlow
	case typicalPrice < m.previousTypicalPrice:
		negFlow = rawMoneyFlow
	}

	// Calculate the prospective rolling window
	// before committing any state.
	start := 0
	if len(m.posFlows) == m.period {
		start = 1
	}

	var posSum, negSum float64

	for i := start; i < len(m.posFlows); i++ {
		posSum += m.posFlows[i]
		negSum += m.negFlows[i]
	}

	posSum += posFlow
	negSum += negFlow

	if math.IsNaN(posSum) ||
		math.IsInf(posSum, 0) ||
		math.IsNaN(negSum) ||
		math.IsInf(negSum, 0) {
		return
	}

	// Calculate the next result before committing.
	ready := len(m.posFlows)+1 >= m.period

	var currentMFI float64

	if ready {
		switch {
		case posSum == 0 && negSum == 0:
			currentMFI = 50

		case negSum == 0:
			currentMFI = 100

		case posSum == 0:
			currentMFI = 0

		case posSum > negSum:
			currentMFI = 100 / (1 + negSum/posSum)

		default:
			ratio := posSum / negSum
			currentMFI = 100 * ratio / (1 + ratio)
		}

		if math.IsNaN(currentMFI) ||
			math.IsInf(currentMFI, 0) ||
			currentMFI < 0 ||
			currentMFI > 100 {
			return
		}
	}

	// Commit only after all validations succeed.
	m.previousTypicalPrice = typicalPrice
	m.lastTimestamp = timestamp

	m.posFlows = append(m.posFlows, posFlow)
	m.negFlows = append(m.negFlows, negFlow)

	if len(m.posFlows) > m.period {
		m.posFlows = m.posFlows[1:]
		m.negFlows = m.negFlows[1:]
	}

	if !ready {
		m.result = MFIResult{
			IsReady:   false,
			Timestamp: timestamp,
		}
		return
	}

	previousMFI := currentMFI
	if m.hasResult {
		previousMFI = m.lastValue
	}

	m.result = MFIResult{
		IsReady:   true,
		Current:   currentMFI,
		Previous:  previousMFI,
		Timestamp: timestamp,
	}

	m.lastValue = currentMFI
	m.hasResult = true
}
