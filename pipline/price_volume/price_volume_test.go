package price_volume

import (
	"math"
	"testing"
	"time"
)

type testCandle struct {
	ts         time.Time
	h, l, c, v float64
}

func (x testCandle) GetTime() time.Time       { return x.ts }
func (x testCandle) GetOpen() float64         { return x.c }
func (x testCandle) GetHigh() float64         { return x.h }
func (x testCandle) GetLow() float64          { return x.l }
func (x testCandle) GetClose() float64        { return x.c }
func (x testCandle) GetVolume() float64       { return x.v }
func (x testCandle) GetOpenInterest() float64 { return 0 }
func TestVWAPSessionReset(t *testing.T) {
	a, _ := NewVolumeWeightedAveragePrice("vwap", VolumeWeightedAveragePriceConfig{})
	t0 := time.Date(2026, 10, 1, 9, 15, 0, 0, time.UTC)
	a.Update(testCandle{t0, 110, 90, 100, 10})
	a.Update(testCandle{t0.Add(time.Minute), 220, 180, 200, 30})
	if math.Abs(a.result.Current-175) > 1e-9 {
		t.Fatalf("want 175 got %v", a.result.Current)
	}
	a.Update(testCandle{t0.Add(24 * time.Hour), 110, 90, 100, 5})
	if math.Abs(a.result.Current-100) > 1e-9 {
		t.Fatalf("session reset want 100 got %v", a.result.Current)
	}
}
func TestAVWAPAnchor(t *testing.T) {
	t0 := time.Date(2026, 10, 1, 9, 15, 0, 0, time.UTC)
	a, _ := NewAnchoredVolumeWeightedAveragePrice("avwap", AnchoredVolumeWeightedAveragePriceConfig{AnchorTime: t0.Add(time.Minute)})
	a.Update(testCandle{t0, 110, 90, 100, 10})
	if a.result.IsReady {
		t.Fatal("pre-anchor must not be ready")
	}
	a.Update(testCandle{t0.Add(time.Minute), 220, 180, 200, 30})
	if !a.result.IsReady || math.Abs(a.result.Current-200) > 1e-9 {
		t.Fatalf("want 200 got %+v", a.result)
	}
}
