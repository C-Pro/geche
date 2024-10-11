package geche

import (
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
	for i := range []int{0, 5, 14} {
		c.SetIfPresent(strconv.Itoa(i), strconv.Itoa(i))
	}

	if _, err := c.Get(strconv.Itoa(0)); err != ErrNotFound {
		t.Errorf("Get(%d): expected ErrNotFound, but got %v", 0, err)
	}

	checkExistingKeys()
}
