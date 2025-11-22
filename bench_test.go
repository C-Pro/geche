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

func benchmarkSetIfPresent(c Geche[string, string], testKeys []string, b *testing.B) {
	for i := 0; i < b.N; i++ {
		c.SetIfPresent(testKeys[i%len(testKeys)], "value")
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
		{
			"KVMapCache",
			NewKV[string](NewMapCache[string, string]()),
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

func BenchmarkSetIfPresentOnlyHits(b *testing.B) {
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
		{
			"KVMapCache",
			NewKV[string](NewMapCache[string, string]()),
		},
	}

	testKeys := make([]string, 10_000_000)
	for i := 0; i < len(testKeys); i++ {
		testKeys[i] = strconv.Itoa(i)
	}

	for _, c := range tab {
		for k := 0; k < len(testKeys); k++ {
			c.imp.Set(testKeys[k], "value")
		}

		b.Run(c.name, func(b *testing.B) {
			benchmarkSetIfPresent(c.imp, testKeys, b)
		})
	}
}

func BenchmarkSetIfPresentOnlyMisses(b *testing.B) {
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
		{
			"KVMapCache",
			NewKV[string](NewMapCache[string, string]()),
		},
	}


	for _, c := range tab {
		for k := 0; k < 10_000_000; k++ {
			c.imp.Set(strconv.Itoa(k), "value")
		}
		b.Run(c.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				c.imp.SetIfPresent("absent", "never set")
			}
		})
	}
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
			NewRingBuffer[string, string](1000000),
		},
		{
			"ShardedRingBufferUpdater",
			NewCacheUpdater[string, string](
				NewSharded[string](
					func() Geche[string, string] { return NewRingBuffer[string, string](100000) },
					0,
					&StringMapper{},
				),
				updateFn,
				8,
			),
		},
		{
			"KVMapCache",
			NewKV[string](NewMapCache[string, string]()),
		},
		{
			"LockerMapCache",
			NewLocker[string, string](NewMapCache[string, string]()).Lock(),
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

func randomString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte('0') + byte(rand.Intn(74))
	}

	return string(b)
}

// This one is quite slow. Trace shows that most of the time is spent allocating output slice.
func BenchmarkKVListByPrefix(b *testing.B) {
	c := NewKV[string](NewMapCache[string, string]())
	keys := make([]string, 100_000)
	for i := 0; i < 100_000; i++ {
		l := rand.Intn(15) + 15
		unique := randomString(l)
		keys[i] = unique
		for j := 0; j < 10; j++ {
			c.Set(unique+randomString(l), randomString(l))
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := c.ListByPrefix(keys[i%len(keys)])
		if err != nil {
			b.Errorf("unexpected error in ListByPrefix: %v", err)
		}
		if len(res) != 10 {
			b.Errorf("expected len 10, but got %d", len(res))
		}
	}
}
