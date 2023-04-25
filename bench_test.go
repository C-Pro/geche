package geche

import (
	"context"
	"math/rand"
	"strconv"
	"testing"
	"time"
)

const keyCardinality = 1000000

type testCase struct {
	key string
	op  int
}

const (
	OPGet = iota
	OPSet
	OPDel
)

func genTestData(N int) []testCase {
	d := make([]testCase, N)
	for i := range d {
		d[i].key = strconv.Itoa(rand.Intn(keyCardinality))
		r := rand.Float64()
		switch {
		case r < 0.9:
			d[i].op = OPGet
		case r >= 0.9 && r < 0.95:
			d[i].op = OPSet
		case r >= 0.95:
			d[i].op = OPDel
		}
	}

	return d
}


func benchmarkSet(c Geche[string, string], testData []testCase, b *testing.B) {
	for i := 0; i < b.N; i++ {
		c.Set(testData[i%len(testData)].key, "value")
	}
}

func benchmarkFuzz(
	c Geche[string, string],
	testData []testCase,
	b *testing.B,
) {
	for i := 0; i < b.N; i++ {
		switch testData[i%len(testData)].op {
		case OPGet:
			_, _ = c.Get(testData[i%len(testData)].key)
		case OPSet:
			c.Set(testData[i%len(testData)].key, "value")
		case OPDel:
			_ = c.Del(testData[i%len(testData)].key)
		}
	}
}

func BenchmarkSet(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tab := []struct {
		name string
		imp  Geche[string, string]
	}{
		{
			"MapCache",
			NewMapCache[string, string](),
		},
		{
			"StringCache",
			newStringCache(),
		},
		{
			"UnsafeCache",
			newUnsafeCache(),
		},
		{
			"MapTTLCache",
			NewMapTTLCache[string, string](ctx, time.Minute, time.Minute),
		},
		{
			"RingBuffer",
			NewRingBuffer[string, string](1000000),
		},
	}

	data := genTestData(10_000_000)
	b.ResetTimer()
	for _, c := range tab {
		b.Run(c.name, func(b *testing.B) {
			benchmarkSet(c.imp, data, b)
		})
	}

	b.Run("AnyCache", func(b *testing.B) {
		c := newAnyCache()
		for i := 0; i < b.N; i++ {
			c.Set(data[i%len(data)].key, "value")
		}
	})
}

// BenchmarkEverything performs different operations randomly.
// Ratio for get/set/del is 90/5/5
func BenchmarkEverything(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tab := []struct {
		name string
		imp  Geche[string, string]
	}{
		{
			"MapCache",
			NewMapCache[string, string](),
		},
		{
			"StringCache",
			newStringCache(),
		},
		{
			"UnsafeCache",
			newUnsafeCache(),
		},
		{
			"MapTTLCache",
			NewMapTTLCache[string, string](ctx, time.Minute, time.Minute),
		},
		{
			"RingBuffer",
			NewRingBuffer[string, string](10000),
		},
	}

	data := genTestData(10_000_000)
	b.ResetTimer()
	for _, c := range tab {
		b.Run(c.name, func(b *testing.B) {
			benchmarkFuzz(c.imp, data, b)
		})
	}

	b.Run("AnyCache", func(b *testing.B) {
		c := newAnyCache()
		for i := 0; i < b.N; i++ {
			key := strconv.Itoa(rand.Intn(keyCardinality))
			r := rand.Float64()
			switch {
			case r < 0.9:
				_, _ = c.Get(key)
			case r >= 0.9 && r < 0.95:
				_ = c.Del(key)
			case r >= 0.95:
				c.Set(key, "value")
			}
		}
	})
}
