package pipline

import (
	"fmt"

	"indicators/pipline/candlestick"
	"indicators/pipline/derivatives"
	"indicators/pipline/momentum"
	"indicators/pipline/price_reference"
	"indicators/pipline/price_volume"
	"indicators/pipline/rules"
	"indicators/pipline/technical"
	"indicators/pipline/trend"

	"indicators/pipline/volatility"
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
		return momentum.NewRSI(rule.Name, rule.Config)

	case "commodityChannelIndex":
		return momentum.NewCCI(rule.Name, rule.Config)

	case "awesomeOscillator":
		return momentum.NewAO(rule.Name, rule.Config)

	case "moneyFlowIndex":
		return momentum.NewMFI(rule.Name, rule.Config)

	case "movingAverageConvergenceDivergence":
		return momentum.NewMACD(rule.Name, rule.Config)

	case "rateOfChange":
		return momentum.NewROC(rule.Name, rule.Config)

	case "stochasticOscillator":
		return momentum.NewStochasticOscillator(rule.Name, rule.Config)

	case "stochasticRSI":
		return momentum.NewStochasticRSI(rule.Name, rule.Config)

	case "williamsR":
		return momentum.NewWilliamsPercentRange(rule.Name, rule.Config)

	case "aroonOscillator":
		return trend.NewAroonOscillator(rule.Name, rule.Config)

	case "aroon":
		return trend.NewAroon(rule.Name, rule.Config)

	case "averageDirectionalIndex":
		return trend.NewADX(rule.Name, rule.Config)

	case "exponentialMovingAverage":
		return trend.NewEMA(rule.Name, rule.Config)

	case "halfTrend":
		return trend.NewHalfTrend(rule.Name, rule.Config)

	case "hullMovingAverage":
		return trend.NewHMA(rule.Name, rule.Config)

	case "ichimokuCloud":
		return trend.NewIchimokuCloud(rule.Name, rule.Config)

	case "parabolicSAR":
		return trend.NewParabolicSAR(rule.Name, rule.Config)

	case "sMA":
		return trend.NewSMA(rule.Name, rule.Config)

	case "smoothedMovingAverage":
		return trend.NewSMMA(rule.Name, rule.Config)

	case "superTrend":
		return trend.NewSupertrend(rule.Name, rule.Config)

	case "trendIntensityIndex":
		return trend.NewTrendIntensityIndex(rule.Name, rule.Config)

	case "weightedMovingAverage":
		return trend.NewWMA(rule.Name, rule.Config)

	case "wilderSmoothingAverage":
		return trend.NewWilderSmoothingAverage(rule.Name, rule.Config)

	case "williamsAlligator":
		return trend.NewWilliamsAlligator(rule.Name, rule.Config)

	case "averageTrueRange":
		return volatility.NewAverageTrueRange(rule.Name, rule.Config)

	case "bollingerBand":
		return volatility.NewBollingerBand(rule.Name, rule.Config)

	case "chandelierExit":
		return volatility.NewChandelierExit(rule.Name, rule.Config)

	case "donChianChannel":
		return volatility.NewDonchianChannel(rule.Name, rule.Config)

	case "keltnerChannel":
		return volatility.NewKeltnerChannel(rule.Name, rule.Config)

	case "meanReversionChannel":
		return volatility.NewMeanReversionChannel(rule.Name, rule.Config)

	case "coppockCurve":
		return technical.NewCoppockCurve(rule.Name, rule.Config)

	case "fisherTransform":
		return technical.NewFisherTransform(rule.Name, rule.Config)

	case "movingAverageForATR":
		return technical.NewMovingAverageForAverageTrueRange(rule.Name, rule.Config)

	case "movingAverageForRSI":
		return technical.NewMovingAverageForRelativeStrengthIndex(rule.Name, rule.Config)

	case "qqeSignals":
		return technical.NewQuantitativeQualitativeEstimationSignals(rule.Name, rule.Config)

	case "anchoredVolumeWeightedAveragePrice":
		return price_volume.NewAnchoredVolumeWeightedAveragePrice(rule.Name, rule.Config)

	case "centralPivotRange":
		return price_volume.NewCentralPivotRange(rule.Name, rule.Config)

	case "openingRangeBreakout":
		return price_volume.NewOpeningRangeBreakout(rule.Name, rule.Config)

	case "pivotPoints":
		return price_volume.NewPivotPoints(rule.Name, rule.Config)

	case "candleAt":
		return price_volume.NewRangeBreakout(rule.Name, rule.Config)

	case "volumeWeightedAveragePrice":
		return price_volume.NewVolumeWeightedAveragePrice(rule.Name, rule.Config)

	case "signal":
		return rules.NewCondition(rule.Name, rule.Config)

	case "currentCandle":
		return price_reference.NewCurrentCandle(rule.Name, rule.Config)

	case "entryCandle":
		return price_reference.NewEntryCandle(rule.Name, rule.Config)

	case "lastNCandles":
		return price_reference.NewLastNCandles(rule.Name, rule.Config)

	case "previousCandle":
		return price_reference.NewPreviousCandle(rule.Name, rule.Config)

	case "yesterdayCandle":
		return price_reference.NewPreviousDayCandle(rule.Name, rule.Config)

	case "longBuildup":
		return derivatives.NewLongBuildup(rule.Name, rule.Config)

	case "longUnwinding":
		return derivatives.NewLongUnwinding(rule.Name, rule.Config)

	case "todayCandle":
		return price_reference.NewTodayCandle(rule.Name, rule.Config)

	case "previousNthCandle":
		return price_reference.NewPreviousNthCandle(rule.Name, rule.Config)

	// case "ratio":
	// 	return derivatives.NewRatio(rule.Name, rule.Config)

	case "shortBuildup":
		return derivatives.NewShortBuildup(rule.Name, rule.Config)

	case "gapStrategy":
		return derivatives.NewGapStrategy(rule.Name, rule.Config)

	case "doji":
		return candlestick.NewDoji(rule.Name, rule.Config)

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
