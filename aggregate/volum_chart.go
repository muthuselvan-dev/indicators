package aggregate

import (
	"fmt"
	"math"
	"time"

	"indicators/core"
)

type VolumeChart struct {
	id             string
	candleInterval time.Duration
	volumeInterval time.Duration

	bucket    time.Time
	current   core.Candle
	count     int
	lastInput time.Time

	result core.MarketCandle
	ready  bool
}

func NewVolumeChart(id string, cfg map[string]any) (*VolumeChart, error) {

	candleIntervalStr, ok := cfg["candleInterval"].(string)
	if !ok {
		return nil, fmt.Errorf("VolumeChart: candleInterval must be a string")
	}

	candleInterval, err := ParseInterval(candleIntervalStr)
	if err != nil {
		return nil, fmt.Errorf("VolumeChart: %w", err)
	}

	volumeIntervalStr, ok := cfg["volumeInterval"].(string)
	if !ok {
		return nil, fmt.Errorf("VolumeChart: volumeInterval must be a string")
	}

	volumeInterval, err := ParseInterval(volumeIntervalStr)
	if err != nil {
		return nil, fmt.Errorf("VolumeChart: %w", err)
	}

	if candleInterval < time.Minute || volumeInterval < time.Minute || candleInterval%time.Minute != 0 ||
		volumeInterval%time.Minute != 0 ||
		volumeInterval > candleInterval {

		return nil, fmt.Errorf("VolumeChart: invalid intervals")
	}

	return &VolumeChart{
		id:             id,
		candleInterval: candleInterval,
		volumeInterval: volumeInterval,
	}, nil
}

func (r *VolumeChart) Update(candle core.MarketCandle) (core.MarketCandle, bool) {

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

func (r *VolumeChart) ingest(candle core.MarketCandle) (core.MarketCandle, bool) {
	t := candle.GetTime()
	y, m, d := t.Date()

	start := time.Date(y, m, d, 9, 15, 0, 0, t.Location())
	end := start.Add(375 * time.Minute)
	minute := t.Truncate(time.Minute)

	if minute.Before(start) || !minute.Before(end) {
		return nil, false
	}

	bucket := start.Add(
		(minute.Sub(start) / r.candleInterval) * r.candleInterval,
	)

	bucketEnd := bucket.Add(r.candleInterval)
	if bucketEnd.After(end) {
		bucketEnd = end
	}

	position := int(minute.Sub(bucket) / time.Minute)
	expected := int(bucketEnd.Sub(bucket) / time.Minute)

	if !bucket.Equal(r.bucket) {
		r.bucket = bucket
		r.count = 0

		r.current = core.Candle{
			Time:         bucket,
			Open:         candle.GetOpen(),
			High:         candle.GetHigh(),
			Low:          candle.GetLow(),
			Close:        candle.GetClose(),
			OpenInterest: candle.GetOpenInterest(),
		}

		// Missing first candle: mark bucket invalid.
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
		r.current.OpenInterest = candle.GetOpenInterest()
		// r.current.Time = t
	}

	elapsed := minute.Sub(bucket)

	if elapsed < r.volumeInterval {
		r.current.Volume += candle.GetVolume()
	}

	if r.count >= 0 {
		r.count++
	}

	if position != expected-1 {
		return nil, false
	}

	completed := r.current
	valid := r.count == expected

	r.count = 0
	r.bucket = time.Time{}

	if !valid {
		return nil, false
	}

	r.result = completed
	r.ready = true

	return r.Result()
}

func (r *VolumeChart) Result() (core.MarketCandle, bool) {
	return r.result, r.ready
}
