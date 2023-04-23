package geche

import (
	"context"
	"strconv"
	"testing"
	"time"
)

func testSetGet(t *testing.T, imp Geche[string, string]) {
	imp.Set("key", "value")
	val, err := imp.Get("key")
	if err != nil {
		t.Errorf("unexpected error in Get: %v", err)
	}

	if val != "value" {
		t.Errorf("expected value %q, got %q", "value", val)
	}
}

func testGetNonExist(t *testing.T, imp Geche[string, string]) {
	_, err := imp.Get("key")
	if err != ErrNotFound {
		t.Errorf("expected error %v, got %v", ErrNotFound, err)
	}
}

func testDel(t *testing.T, imp Geche[string, string]) {
	imp.Set("key", "value")

	if err := imp.Del("key"); err != nil {
		t.Errorf("unexpected error in Del: %v", err)
	}

	_, err := imp.Get("key")
	if err != ErrNotFound {
		t.Errorf("expected error %v, got %v", ErrNotFound, err)
	}
}

func testDelOdd(t *testing.T, imp Geche[string, string]) {
	for i := 0; i < 100; i++ {
		s := strconv.Itoa(i)
		imp.Set(s, s)
	}

	// Check we can get all 100 values correctly.
	for i := 0; i < 100; i++ {
		s := strconv.Itoa(i)
		val, err := imp.Get(s)
		if err != nil {
			t.Errorf("unexpected error in Get: %v", err)
		}

		if val != s {
			t.Errorf("expected value %q, got %q", "value", val)
		}
	}

	for i := 0; i < 100; i++ {
		if i%2 == 0 {
			continue
		}
		// Delete odd keys.
		s := strconv.Itoa(i)
		if err := imp.Del(s); err != nil {
			t.Errorf("unexpected error in Del: %v", err)
		}
	}

	// Check odd values don't exist, while even do exist.
	for i := 0; i < 100; i++ {
		s := strconv.Itoa(i)
		val, err := imp.Get(s)
		if i%2 == 0 {
			if err != nil {
				t.Errorf("unexpected error in Get: %v", err)
			}

			if val != s {
				t.Errorf("expected value %q, got %q", "value", val)
			}

			continue
		}

		if err != ErrNotFound {
			t.Errorf("expected error %v, got %v", ErrNotFound, err)
		}
	}

	if err := imp.Del("key"); err != nil {
		t.Errorf("unexpected error in Del: %v", err)
	}

	_, err := imp.Get("key")
	if err != ErrNotFound {
		t.Errorf("expected error %v, got %v", ErrNotFound, err)
	}
}

// TestCommon runs a common set of tests on all implementations of Geche interface.
func TestCommon(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	caches := []struct {
		name    string
		factory func() Geche[string, string]
	}{
		{"MapCache", func() Geche[string, string] { return NewMapCache[string, string]() }},
		{"MapTTLCache", func() Geche[string, string] { return NewMapTTLCache[string, string](ctx, time.Minute, time.Minute) }},
	}

	tab := []struct {
		name    string
		theTest func(*testing.T, Geche[string, string])
	}{
		{"SetGet", testSetGet},
		{"GetNonExist", testGetNonExist},
		{"Del", testDel},
		{"DelOdd", testDelOdd},
	}
	for _, ci := range caches {
		for _, tc := range tab {
			t.Run(ci.name+tc.name, func(t *testing.T) {
				tc.theTest(t, ci.factory())
			})
		}
	}
}
