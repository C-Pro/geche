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
