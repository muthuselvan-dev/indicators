package aggregate

import (
	"fmt"
	"indicators/core"
	"strconv"
	"strings"
	"time"
)

// NSE market timings
const (
	marketOpenHour    = 9
	marketOpenMinute  = 15
	marketCloseHour   = 15
	marketCloseMinute = 29 // last 1-min candle
)

type Aggregator struct {
	interval      int
	currentBucket time.Time
	current       *core.Candle
}

// compile-time check
var _ core.MarketCandle = core.Candle{}

// ParseInterval: "5 minutes" -> 5, "1 hour" -> 60
func ParseInterval(s string) (int, error) {
	parts := strings.Fields(strings.ToLower(strings.TrimSpace(s)))
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid candleInterval %q, expected like \"5 minutes\"", s)
	}

	n, err := strconv.Atoi(parts[0])
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid candleInterval number in %q", s)
	}

	switch parts[1] {
	case "minute", "minutes", "min", "mins", "m":
		return n, nil
	case "hour", "hours", "h":
		return n * 60, nil
	default:
		return 0, fmt.Errorf("invalid candleInterval unit in %q", s)
	}
}

// Aggregator merges 1-minute candles into N-minute candles.

func NewAggregator(interval int) *Aggregator {
	if interval < 1 {
		interval = 1
	}
	return &Aggregator{interval: interval}
}

func NewAggregatorFromString(s string) (*Aggregator, error) {
	n, err := ParseInterval(s)
	if err != nil {
		return nil, err
	}
	return NewAggregator(n), nil
}

func (a *Aggregator) bucketStart(t time.Time) time.Time {
	open := time.Date(t.Year(), t.Month(), t.Day(),
		marketOpenHour, marketOpenMinute, 0, 0, t.Location())

	t = t.Truncate(time.Minute) // seconds ignore

	minutesFromOpen := int(t.Sub(open).Minutes())
	idx := floorDiv(minutesFromOpen, a.interval)

	return open.Add(time.Duration(idx*a.interval) * time.Minute)
}

func floorDiv(a, b int) int {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

// Aggregate returns (aggregatedCandle, true) when a bucket is complete.
func (a *Aggregator) Aggregate(c core.MarketCandle) (core.MarketCandle, bool) {
	// interval 1 -> direct pass
	if a.interval == 1 {
		return c, true
	}

	bucket := a.bucketStart(c.GetTime())

	// first candle ever
	if a.current == nil {
		a.startBucket(bucket, c)
		return a.checkComplete(c)
	}

	// new bucket vanthuduchu (last minute candle miss aana case)
	if !bucket.Equal(a.currentBucket) {
		done := *a.current
		a.startBucket(bucket, c)
		return done, true
	}

	// same bucket -> merge
	a.merge(c)
	return a.checkComplete(c)
}

func (a *Aggregator) startBucket(bucket time.Time, c core.MarketCandle) {
	a.current = &core.Candle{
		Time:         bucket,
		Open:         c.GetOpen(),
		High:         c.GetHigh(),
		Low:          c.GetLow(),
		Close:        c.GetClose(),
		Volume:       c.GetVolume(),
		OpenInterest: c.GetOpenInterest(),
	}
	a.currentBucket = bucket
}

func (a *Aggregator) merge(c core.MarketCandle) {
	cur := a.current
	if c.GetHigh() > cur.High {
		cur.High = c.GetHigh()
	}
	if c.GetLow() < cur.Low {
		cur.Low = c.GetLow()
	}
	cur.Close = c.GetClose()
	cur.Volume += c.GetVolume()
	cur.OpenInterest = c.GetOpenInterest()
}

// checkComplete: bucket-oda last minute candle ah nu check pannum
func (a *Aggregator) checkComplete(c core.MarketCandle) (core.MarketCandle, bool) {
	lastMinute := a.currentBucket.Add(time.Duration(a.interval-1) * time.Minute)

	// bucket end market close ku mela poga koodathu
	closeCandle := time.Date(a.currentBucket.Year(), a.currentBucket.Month(), a.currentBucket.Day(),
		marketCloseHour, marketCloseMinute, 0, 0, a.currentBucket.Location())
	if lastMinute.After(closeCandle) {
		lastMinute = closeCandle
	}

	if c.GetTime().Truncate(time.Minute).Before(lastMinute) {
		return nil, false
	}

	done := *a.current
	a.current = nil
	return done, true
}
