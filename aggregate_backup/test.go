package aggregate

import (
	"testing"
	"time"
)

type fc struct {
	t                 time.Time
	o, h, l, c, v, oi float64
}

func (f fc) GetTime() time.Time       { return f.t }
func (f fc) GetOpen() float64         { return f.o }
func (f fc) GetHigh() float64         { return f.h }
func (f fc) GetLow() float64          { return f.l }
func (f fc) GetClose() float64        { return f.c }
func (f fc) GetVolume() float64       { return f.v }
func (f fc) GetOpenInterest() float64 { return f.oi }

func TestFiveMinute(t *testing.T) {
	a := NewAggregator(5)
	base := time.Date(2025, 8, 1, 9, 15, 59, 0, time.Local)

	rows := []fc{
		{base, 56200, 56200, 56000, 56062.2, 28770, 1},
		{base.Add(1 * time.Minute), 56053.8, 56073.2, 56030.2, 56040.0, 8995, 1},
		{base.Add(2 * time.Minute), 56044.4, 56044.4, 56030.2, 56030.4, 6825, 1},
		{base.Add(3 * time.Minute), 56028.0, 56035.4, 56010.0, 56011.0, 5285, 1},
		{base.Add(4 * time.Minute), 56010.0, 56044.0, 56005.0, 56035.0, 9205, 2},
	}

	for i, r := range rows {
		got, ready := a.Aggregate(r)
		if i < 4 && ready {
			t.Fatalf("candle %d: ready too early", i)
		}
		if i == 4 {
			if !ready {
				t.Fatal("expected ready on 5th candle")
			}
			if got.GetOpen() != 56200 || got.GetHigh() != 56200 ||
				got.GetLow() != 56000 || got.GetClose() != 56035 ||
				got.GetVolume() != 59080 {
				t.Fatalf("wrong candle: %+v", got)
			}
		}
	}
}

func TestThirtyMinuteLastBucket(t *testing.T) {
	a := NewAggregator(30)
	base := time.Date(2025, 8, 1, 15, 15, 59, 0, time.Local)

	var ready bool
	for i := 0; i < 15; i++ { // 15:15 to 15:29
		_, ready = a.Aggregate(fc{base.Add(time.Duration(i) * time.Minute), 100, 101, 99, 100, 1, 1})
		if i < 14 && ready {
			t.Fatalf("candle %d: ready too early", i)
		}
	}
	if !ready {
		t.Fatal("expected flush at 15:29")
	}
}
