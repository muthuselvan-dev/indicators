package aggregate

import (
	"fmt"
	"math"
	"time"

	"indicators/core"
)

type HeikinChart struct {
	id             string
	candleInterval time.Duration

	active    bool
	bucket    time.Time
	current   core.Candle
	count     int
	lastInput time.Time

	previousHAOpen  float64
	previousHAClose float64
	hasPreviousHA   bool

	result core.MarketCandle
	ready  bool
}

func NewHeikinChart(id string, cfg map[string]any) (*HeikinChart, error) {
	candleInterval := cfg["candleInterval"].(string)

	duration, err := ParseInterval(candleInterval)
	if err != nil {
		return nil, fmt.Errorf("HeikinChart: %w", err)
	}

	return &HeikinChart{
		id:             id,
		candleInterval: duration,
	}, nil
}

func (r *HeikinChart) Update(candle core.MarketCandle) (core.MarketCandle, bool) {
	r.result = nil
	r.ready = false

	if candle == nil || candle.GetTime().IsZero() {
		return nil, false
	}

	t := candle.GetTime()

	if !r.lastInput.IsZero() && !t.After(r.lastInput) {
		return nil, false
	}

	o := candle.GetOpen()
	h := candle.GetHigh()
	l := candle.GetLow()
	c := candle.GetClose()
	v := candle.GetVolume()
	oi := candle.GetOpenInterest()

	for _, value := range [...]float64{o, h, l, c, v, oi} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, false
		}
	}

	if o <= 0 || h < l || h < math.Max(o, c) ||
		l > math.Min(o, c) || v < 0 || oi < 0 {
		return nil, false
	}

	r.lastInput = t
	return r.ingest(candle)
}

func (r *HeikinChart) ingest(candle core.MarketCandle) (core.MarketCandle, bool) {
	t := candle.GetTime()
	y, m, d := t.Date()

	sessionStart := time.Date(y, m, d, 9, 15, 0, 0, t.Location())
	sessionEnd := sessionStart.Add(375 * time.Minute)
	minute := t.Truncate(time.Minute)

	if minute.Before(sessionStart) || !minute.Before(sessionEnd) {
		return nil, false
	}

	bucket := sessionStart.Add(
		(minute.Sub(sessionStart) / r.candleInterval) * r.candleInterval,
	)

	bucketEnd := bucket.Add(r.candleInterval)
	if bucketEnd.After(sessionEnd) {
		bucketEnd = sessionEnd
	}

	position := int(minute.Sub(bucket) / time.Minute)
	expected := int(bucketEnd.Sub(bucket) / time.Minute)

	if !r.active || !bucket.Equal(r.bucket) {
		r.active = true
		r.bucket = bucket
		r.count = 0

		r.current = core.Candle{
			Time:         bucket,
			Open:         candle.GetOpen(),
			High:         candle.GetHigh(),
			Low:          candle.GetLow(),
			Close:        candle.GetClose(),
			Volume:       candle.GetVolume(),
			OpenInterest: candle.GetOpenInterest(),
		}

		if position != 0 {
			r.count = -1
		}
	} else {
		if r.count >= 0 && position != r.count {
			r.count = -1
		}

		r.current.High = math.Max(r.current.High, candle.GetHigh())
		r.current.Low = math.Min(r.current.Low, candle.GetLow())
		r.current.Close = candle.GetClose()
		r.current.Volume += candle.GetVolume()
		r.current.OpenInterest = candle.GetOpenInterest()
		// r.current.Time = t
	}

	if r.count >= 0 {
		r.count++
	}

	if position != expected-1 {
		return nil, false
	}

	raw := r.current
	valid := r.count == expected

	r.active = false
	r.count = 0

	if !valid {
		return nil, false
	}

	haClose := (raw.Open + raw.High + raw.Low + raw.Close) / 4
	haOpen := (raw.Open + raw.Close) / 2

	if r.hasPreviousHA {
		haOpen = (r.previousHAOpen + r.previousHAClose) / 2
	}

	haHigh := math.Max(raw.High, math.Max(haOpen, haClose))
	haLow := math.Min(raw.Low, math.Min(haOpen, haClose))

	r.previousHAOpen = haOpen
	r.previousHAClose = haClose
	r.hasPreviousHA = true

	r.result = core.Candle{
		Time:         raw.Time,
		Open:         haOpen,
		High:         haHigh,
		Low:          haLow,
		Close:        haClose,
		Volume:       raw.Volume,
		OpenInterest: raw.OpenInterest,
	}

	r.ready = true
	return r.Result()
}

func (r *HeikinChart) Result() (core.MarketCandle, bool) {
	return r.result, r.ready
}
