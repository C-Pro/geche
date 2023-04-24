package geche

import (
	"context"
	"math/rand"
	"strconv"
	"testing"
	"time"
)

func benchmarkSet(c Geche[string, string], b *testing.B) {
	for i := 0; i < b.N; i++ {
		c.Set(strconv.Itoa(i), strconv.Itoa(i))
	}
}

func benchmarkFuzz(c Geche[string, string], b *testing.B) {
	for i := 0; i < b.N; i++ {
		key := strconv.Itoa(rand.Int())
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
			NewRingBuffer[string, string](10000),
		},
	}
	for _, c := range tab {
		b.Run(c.name, func(b *testing.B) {
			benchmarkSet(c.imp, b)
		})
	}

	b.Run("AnyCache", func(b *testing.B) {
		c := newAnyCache()
		for i := 0; i < b.N; i++ {
			c.Set(strconv.Itoa(i), strconv.Itoa(i))
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
	for _, c := range tab {
		b.Run(c.name, func(b *testing.B) {
			benchmarkFuzz(c.imp, b)
		})
	}

	b.Run("AnyCache", func(b *testing.B) {
		c := newAnyCache()
		for i := 0; i < b.N; i++ {
			key := strconv.Itoa(rand.Int())
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
