package geche

import (
	"strconv"
	"testing"
)

func TestSharded(t *testing.T) {
	c := NewSharded[int](
		func() Geche[int, string] {
			return NewMapCache[int, string]()
		},
		4,
		&NumberMapper[int]{},
	)

	for i := 0; i < 1000; i++ {
		c.Set(i, strconv.Itoa(i))
	}

	for i := 0; i < 1000; i++ {
		v, err := c.Get(i)
		if err != nil {
			t.Fatalf("unexpected error in Get: %v", err)
		}

		if v != strconv.Itoa(i) {
			t.Fatalf("key %d with unexpected value %q", i, v)
		}
	}

	if len(c.shards) != 4 {
		t.Errorf("expected number of shards to be 4 but got %d", len(c.shards))
	}

	
}
