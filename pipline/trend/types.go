package trend

import "time"

type SMAResult struct {
	IsReady   bool
	Current   float64
	Previous  float64
	Timestamp time.Time
}

type EMAResult struct {
	IsReady   bool
	Current   float64
	Previous  float64
	Timestamp time.Time
}

type WMAResult struct {
	IsReady   bool
	Current   float64
	Previous  float64
	Timestamp time.Time
}

type HMAResult struct {
	IsReady   bool
	Current   float64
	Previous  float64
	Timestamp time.Time
}

type SMMAResult struct {
	IsReady   bool
	Current   float64
	Previous  float64
	Timestamp time.Time
}

type WilderSmoothingAverageResult struct {
	IsReady   bool
	Current   float64
	Previous  float64
	Timestamp time.Time
}

type SupertrendResult struct {
	IsReady   bool
	Current   float64
	Previous  float64
	Timestamp time.Time
	UpperBand float64
	LowerBand float64
	Direction int
}

type ADXResult struct {
	IsReady   bool
	Current   float64
	Previous  float64
	Timestamp time.Time
	PlusDI    float64
	MinusDI   float64
}

type AroonResult struct {
	IsReady   bool
	Current   float64
	Previous  float64
	Timestamp time.Time
	Up        float64
	Down      float64
}

type AroonOscillatorResult struct {
	IsReady   bool
	Current   float64
	Previous  float64
	Timestamp time.Time
}

type ParabolicSARResult struct {
	IsReady   bool
	Current   float64
	Previous  float64
	Timestamp time.Time
	Direction int
}

type IchimokuCloudResult struct {
	IsReady    bool
	Current    float64
	Previous   float64
	Timestamp  time.Time
	Conversion float64
	Base       float64
	SpanA      float64
	SpanB      float64
}

type HalfTrendResult struct {
	IsReady   bool
	Current   float64
	Previous  float64
	Timestamp time.Time
}

type TrendIntensityIndexResult struct {
	IsReady   bool
	Current   float64
	Previous  float64
	Timestamp time.Time
}

type WilliamsAlligatorResult struct {
	IsReady   bool
	Current   float64
	Previous  float64
	Timestamp time.Time
	Jaw       float64
	Teeth     float64
	Lips      float64
}
