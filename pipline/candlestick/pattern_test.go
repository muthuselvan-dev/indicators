package candlestick

import (
	"testing"
	"time"
)

type testCandle struct {
	t                 time.Time
	o, h, l, c, v, oi float64
}

func (x testCandle) GetTime() time.Time       { return x.t }
func (x testCandle) GetOpen() float64         { return x.o }
func (x testCandle) GetHigh() float64         { return x.h }
func (x testCandle) GetLow() float64          { return x.l }
func (x testCandle) GetClose() float64        { return x.c }
func (x testCandle) GetVolume() float64       { return x.v }
func (x testCandle) GetOpenInterest() float64 { return x.oi }
func TestBullishEngulfing(t *testing.T) {
	p, _ := NewBullishEngulfing("test", BullishEngulfingConfig{})
	now := time.Date(2026, 1, 1, 9, 15, 0, 0, time.UTC)
	p.Update(testCandle{t: now, o: 100, h: 101, l: 94, c: 95})
	got := p.Update(testCandle{t: now.Add(time.Minute), o: 94, h: 103, l: 93, c: 102}).(PatternResult)
	if !got.IsReady || !got.Current {
		t.Fatalf("expected bullish engulfing: %+v", got)
	}
}
func TestDoji(t *testing.T) {
	p, _ := NewDoji("test", DojiConfig{Tolerance: 0.1})
	now := time.Date(2026, 1, 1, 9, 15, 0, 0, time.UTC)
	got := p.Update(testCandle{t: now, o: 100, h: 105, l: 95, c: 100.1}).(PatternResult)
	if !got.IsReady || !got.Current {
		t.Fatalf("expected doji: %+v", got)
	}
}
