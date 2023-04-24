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

	// Check the value does not exist (overwriten).
	for i := 0; i < 5; i++ {
		s := strconv.Itoa(i)
		if _, err := c.Get(s); err != ErrNotFound {
			t.Errorf("Get(%q): expected error %v, but got %v", s, ErrNotFound, err)
		}
	}

	// Check we can get the value.
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
