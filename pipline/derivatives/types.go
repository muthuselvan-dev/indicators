package derivatives

import "time"

// ScalarResult records the latest valid value and the preceding ready value.
type ScalarResult struct {
	IsReady   bool
	Current   float64
	Previous  float64
	Timestamp time.Time
}

type PositionClassificationResult = ScalarResult

type BasketLegResult struct {
	LegIndex         int
	Type             string
	Side             string
	InstrumentType   string
	InstrumentIndex  int
	OptionType       string
	ExpiryIndex      int
	ExpiryCycle      string
	StrikeType       string
	StrikeIndex      int
	StrikeMultiplier float64
	QuantityType     string
	QuantityValue    float64
	StopGainType     string
	StopGainValue    float64
	StopLossType     string
	StopLossValue    float64
	ReEntryCount     int
	ReExecuteCount   int
}

type BasketResult struct {
	IsReady   bool
	Current   bool
	Previous  bool
	Timestamp time.Time
	Legs      []map[string]any
}

type LongBuildupResult struct {
	IsReady            bool
	Current            bool
	Previous           bool
	PriceChange        float64
	OpenInterestChange float64
	Timestamp          time.Time
}
type LongUnwindingResult struct {
	IsReady            bool
	Current            bool
	Previous           bool
	PriceChange        float64
	OpenInterestChange float64
	Timestamp          time.Time
}

type RatioResult struct {
	IsReady   bool
	Current   float64
	Previous  float64
	Timestamp time.Time
}

type ShortBuildupResult struct {
	IsReady            bool
	Current            bool
	Previous           bool
	PriceChange        float64
	OpenInterestChange float64
	Timestamp          time.Time
}

type GapStrategyResult struct {
	IsReady          bool
	Current          float64
	Previous         float64
	GapPoints        float64
	GapPercent       float64
	GapUp            bool
	GapDown          bool
	FullGapUp        bool
	FullGapDown      bool
	PartialGapUp     bool
	PartialGapDown   bool
	TodayOpen        float64
	PreviousDayClose float64
	PreviousDayHigh  float64
	PreviousDayLow   float64
	Timestamp        time.Time
}

type PriceRationResult struct {
	IsReady   bool
	Current   float64
	Previous  float64
	Timestamp time.Time
}

// TransactionStopLossTrail holds linear trailing settings.
type TransactionStopLossTrail struct {
	Type          string
	AdjustValue   float64
	TrailingValue float64
}

// TransactionStopLossValue holds stop-loss settings.
type TransactionStopLossValue struct {
	Type  string
	Value float64
	Trail TransactionStopLossTrail
}

// TransactionStopLossConfig holds QuantMan-style configuration.
type TransactionStopLossConfig struct {
	CaseIndex string
	StopLoss  TransactionStopLossValue
}

// TransactionStopLossResult holds the latest evaluated state.
type TransactionStopLossResult struct {
	IsReady       bool
	Current       bool
	Previous      bool
	StopLossPrice float64
	CurrentPrice  float64
	EntryPrice    float64
	ProfitPercent float64
	TrailingSteps int
	Timestamp     time.Time
}

// TransactionStopLoss holds transaction state.
type TransactionStopLoss struct {
	id        string
	config    TransactionStopLossConfig
	active    bool
	side      string
	entry     float64
	bestPrice float64
	stopPrice float64
	result    TransactionStopLossResult
}
