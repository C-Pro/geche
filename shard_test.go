package geche

import (
	"fmt"
	"strconv"
	"sync"
	"testing"
)

func ExampleNewSharded() {
	numShards := 4
	c := NewSharded[int](
		func() Geche[int, string] {
			return NewMapCache[int, string]()
		},
		numShards,
		&NumberMapper[int]{},
	)

	// Set 4000 records in 4 parallel goroutines.
	wg := sync.WaitGroup{}
	wg.Add(numShards)
	for i := 0; i < numShards; i++ {
		go func(i int) {
			defer wg.Done()
			for j := i * 1000; j < i*1000+1000; j++ {
				c.Set(j, strconv.Itoa(j))
			}
		}(i)
	}

	wg.Wait()

	for i := 0; i < 10; i++ {
		v, _ := c.Get(i*1000 + 500)
		fmt.Println(v)
	}

	// Output: 500
	// 1500
	// 2500
	// 3500
}

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

	for i := 0; i < 1000; i++ {
		old, inserted := c.SetIfPresent(i, "modified")
		if !inserted {
			t.Fatalf("SetIfPresent returned inserted=false for existing key %d", i)
		}

		if old != strconv.Itoa(i) {
			t.Fatalf("SetIfPresent returned unexpected old value, expected=%d, got=%s", i, old)
		}
	}

	for i := 1000; i < 2000; i++ {
		_, inserted := c.SetIfPresent(i, "modified")
		if inserted {
			t.Fatalf("SetIfPresent returned inserted=true for non-existing key %d", i)
		}
	}

	for i := 0; i < 1000; i++ {
		v, err := c.Get(i)
		if err != nil {
			t.Fatalf("unexpected error in Get: %v", err)
		}

		if v != "modified" {
			t.Fatalf("key %d with unexpected value %q", i, v)
		}
	}

	for i := 1000; i < 2000; i++ {
		_, err := c.Get(i)
		if err == nil {
			t.Fatalf("expected error in Get")
		}
	}

	if len(c.shards) != 4 {
		t.Errorf("expected number of shards to be 4 but got %d", len(c.shards))
	}
}
