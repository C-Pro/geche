package geche

import (
	"math/rand"
	"strconv"
	"testing"
)

func TestRing(t *testing.T) {
	c := NewRingBuffer[string, string](10)

	for i := 0; i < 15; i++ {
		s := strconv.Itoa(i)
		c.Set(s, s)
	}

	// Check the value does not exist (overwritten).
	for i := 0; i < 5; i++ {
		s := strconv.Itoa(i)
		if _, err := c.Get(s); err != ErrNotFound {
			t.Errorf("Get(%q): expected error %v, but got %v", s, ErrNotFound, err)
		}
	}

	expected := []struct {
		K string
		V string
	}{
		{"5", "5"},
		{"6", "6"},
		{"7", "7"},
		{"8", "8"},
		{"9", "9"},
		{"10", "10"},
		{"11", "11"},
		{"12", "12"},
		{"13", "13"},
		{"14", "14"},
	}

	got := c.ListAll()
	if len(got) != len(expected) {
		t.Errorf("expected %d items, but got %d", len(expected), len(got))
	}

	for i, item := range got {
		if item.K != expected[i].K || item.V != expected[i].V {
			t.Errorf(
				"expected item %s:%s, but got %s:%s",
				expected[i].K, expected[i].V,
				item.K, item.V,
			)
		}
	}

	// Check we can get the value.
	checkExistingKeys := func() {
		for i := 5; i < 15; i++ {
			s := strconv.Itoa(i)
			val, err := c.Get(s)
			if err != nil {
				t.Errorf("unexpected error in Get(%q): %v", s, err)
			}

			if val != s {
				t.Errorf("expected value %q, but got %q", s, val)
			}
		}
	}
	checkExistingKeys()

	// SetIfPresent on existing key or non-existing key does not result in eviction
	_, inserted := c.SetIfPresent(strconv.Itoa(0), strconv.Itoa(0))
	if inserted {
		t.Error("SetIfPresent returned inserted=true for non-existing key 0")
	}

	if _, err := c.Get(strconv.Itoa(0)); err != ErrNotFound {
		t.Errorf("Get(%d): expected ErrNotFound, but got %v", 0, err)
	}

	for _, i := range []int{5, 6, 14} {
		old, inserted := c.SetIfPresent(strconv.Itoa(i), strconv.Itoa(i))
		if !inserted {
			t.Error("SetIfPresent returned inserted=false for existing key")
		}

		if old != strconv.Itoa(i) {
			t.Errorf("SetIfPresent returned incorrect old value, expected %d, got %s", i, old)
		}
	}

	checkExistingKeys()
}

func TestRingListAll(t *testing.T) {
	c := NewRingBuffer[int, int](10)

	// This will emulate the expected behavior of the ring buffer.
	slice := make([]int, 0, 10)

	for i := 0; i < 10000; i++ {
		c.Set(i, i)

		slice = append(slice, i)
		if len(slice) > 10 {
			slice = slice[1:]
		}

		if rand.Intn(10) == 0 {
			// Randomly remove an item from the ring buffer.
			toDelIdx := rand.Intn(len(slice))
			toDelKey := slice[toDelIdx]
			c.Del(toDelKey)
			// Mark item as deleted in the slice.
			// Acrually deleting from the slice wont'b reflect in the ring buffer,
			// as deleted value still takes space in the ring buffer.
			slice[toDelIdx] = -1
		}

		expected := make([]BufferRec[int, int], 0, len(slice))
		for _, v := range slice {
			if v == -1 {
				continue
			}
			expected = append(expected, BufferRec[int, int]{K: v, V: v})
		}

		got := c.ListAll()
		if len(got) != len(expected) {
			t.Fatalf("expected %d items, but got %d", len(expected), len(got))
		}

		for j, item := range got {
			if item.K != expected[j].K || item.V != expected[j].V {
				t.Fatalf(
					"expected item %d:%d, but got %d:%d",
					expected[j].K, expected[j].V,
					item.K, item.V,
				)
			}
		}
	}
}
