package pipline

import (
	"fmt"

	"indicators/pipline/momentum"
)

type Pipeline struct {
	indicators map[string]Indicator
}

func NewPipeline(params []Rule) (*Pipeline, error) {
	pl := &Pipeline{
		indicators: make(map[string]Indicator, len(params)),
	}

	for _, rule := range params {
		if rule.Name == "" {
			return nil, fmt.Errorf("indicator name cannot be empty")
		}

		if _, exists := pl.indicators[rule.Name]; exists {
			return nil, fmt.Errorf("duplicate indicator name: %s", rule.Name)
		}

		indicator, err := pl.loadRule(rule)
		if err != nil {
			return nil, err
		}

		pl.indicators[rule.Name] = indicator
	}

	return pl, nil
}

func (p *Pipeline) loadRule(rule Rule) (Indicator, error) {
	switch rule.Type {
	case "relativeStrengthIndex":
		return momentum.NewRSI(rule.Name, momentum.RSIConfig{
			Period:         rule.Config["period"].(int),
			CandleInterval: rule.Config["candleInterval"].(string),
		})

	case "commodityChannelIndex":
		return momentum.NewCCI(rule.Name, rule.Config)

	case "awesomeOscillator":
		return momentum.NewAO(rule.Name, rule.Config)

	default:
		return nil, fmt.Errorf("unsupported indicator type: %s", rule.Type)
	}
}

func (p *Pipeline) Process(candle MarketCandle) (map[string]any, error) {
	if p == nil {
		return nil, fmt.Errorf("pipeline is nil")
	}

	if candle == nil {
		return nil, fmt.Errorf("market candle is nil")
	}

	results := make(map[string]any, len(p.indicators))

	for name, indicator := range p.indicators {
		results[name] = indicator.Update(candle)
	}

	return results, nil
}
