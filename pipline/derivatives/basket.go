package derivatives

import (
	"fmt"
	"indicators/core"
	"time"
)

// describe: Basket
//
// pipeConfigs: [
//   {
//     type: 'basket',
//     name: 'basket',
//     config: {
//       legs: [...],
//       resetOnDayStart: false,
//       when: [
//         {
//           leftOperand: 'timeOfDay',
//           operator: 'isAbove',
//           rightOperand: '09:15:59',
//           conditionOperator: 'and'
//         }
//       ]
//     }
//   }
// ]
//
// Available result:
//   - Basket condition status
//   - Configured legs
//
// Current = whether the basket entry condition is true.
// Previous = previous candle's condition status.
//
// This implementation evaluates the entry condition.
// It does not execute option orders or calculate P&L.

type Basket struct {
	id string

	legs            []map[string]any
	when            []map[string]any
	resetOnDayStart bool

	lastTimestamp time.Time
	lastDay       string

	result BasketResult
}

func NewBasket(id string, cfg map[string]interface{}) (*Basket, error) {
	if id == "" {
		return nil, fmt.Errorf("Basket name cannot be empty")
	}

	legs, ok := cfg["legs"].([]any)
	if !ok || len(legs) == 0 {
		return nil, fmt.Errorf("Basket legs cannot be empty")
	}

	when, ok := cfg["when"].([]any)
	if !ok || len(when) == 0 {
		return nil, fmt.Errorf("Basket when condition cannot be empty")
	}

	b := &Basket{
		id: id,
	}

	for _, value := range legs {
		leg, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("Basket invalid leg config")
		}

		b.legs = append(b.legs, leg)
	}

	for _, value := range when {
		condition, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("Basket invalid when condition")
		}

		b.when = append(b.when, condition)
	}

	b.resetOnDayStart, _ = cfg["resetOnDayStart"].(bool)

	return b, nil
}

func (b *Basket) Update(candle core.MarketCandle) any {
	b.ingest(candle)
	return b.result
}

func (b *Basket) Result() any {
	return b.result
}

func (b *Basket) ingest(candle core.MarketCandle) {
	if candle == nil {
		return
	}

	timestamp := candle.GetTime()

	if timestamp.IsZero() ||
		(!b.lastTimestamp.IsZero() &&
			!timestamp.After(b.lastTimestamp)) {

		b.result = BasketResult{
			IsReady:   false,
			Timestamp: timestamp,
		}
		return
	}

	day := timestamp.Format("2006-01-02")

	if day != b.lastDay {
		b.lastDay = day
	}

	b.lastTimestamp = timestamp

	current := b.evaluateWhen(timestamp)

	previous := current
	if b.result.IsReady {
		previous = b.result.Current
	}

	b.result = BasketResult{
		IsReady:   true,
		Current:   current,
		Previous:  previous,
		Timestamp: timestamp,
		Legs:      b.legs,
	}
}

func (b *Basket) evaluateWhen(timestamp time.Time) bool {
	result := true

	for _, condition := range b.when {
		leftOperand, _ := condition["leftOperand"].(string)
		operator, _ := condition["operator"].(string)
		rightOperand, _ := condition["rightOperand"].(string)

		if leftOperand != "timeOfDay" {
			return false
		}

		target, err := time.Parse("15:04:05", rightOperand)
		if err != nil {
			return false
		}

		currentSeconds := timestamp.Hour()*3600 +
			timestamp.Minute()*60 +
			timestamp.Second()

		targetSeconds := target.Hour()*3600 +
			target.Minute()*60 +
			target.Second()

		var matched bool

		switch operator {
		case "isAbove":
			matched = currentSeconds > targetSeconds

		case "isBelow":
			matched = currentSeconds < targetSeconds

		case "equalTo":
			matched = currentSeconds == targetSeconds

		case "equalOrAbove":
			matched = currentSeconds >= targetSeconds

		case "equalOrBelow":
			matched = currentSeconds <= targetSeconds

		default:
			return false
		}

		if !matched {
			result = false
			break
		}
	}

	return result
}
