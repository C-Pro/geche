package geche

import (
	"strconv"
	"testing"
)

func benchmarkSet(c Geche[string, string], b *testing.B) {
	for i := 0; i < b.N; i++ {
		c.Set(strconv.Itoa(i), strconv.Itoa(i))
	}
}

func BenchmarkSet(b *testing.B) {
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
