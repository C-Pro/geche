package geche

import (
	"context"
	"strconv"
	"testing"
	"time"
)

func TestTTL(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := NewMapTTLCache[string, string](ctx, time.Second, time.Second)
	c.Set("key", "value")
	ts := time.Now()

	// Check we can get the value.
	val, err := c.Get("key")
	if err != nil {
		t.Errorf("unexpected error in Get: %v", err)
	}

	if val != "value" {
		t.Errorf("expected value %q, but got %v", "value", val)
	}

	// Yep, accessing private variables in the test. So what?
	c.mux.Lock()
	c.now = func() time.Time {
		return ts.Add(time.Second)
	}
	c.mux.Unlock()

	// Check the value does not exist.
	if _, err := c.Get("key"); err != ErrNotFound {
		t.Errorf("expected error %v, but got %v", ErrNotFound, err)
	}

	// Outdated value is still in the cache until cleanup is called.
	if len(c.data) != 1 {
		t.Errorf("expected cache data len to be 1 but got %d", len(c.data))
	}
}

// TestTTLSequence checks linked list works as expected.
func TestTTLSequence(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := NewMapTTLCache[string, string](ctx, time.Second, 0)
	for i := 0; i < 10; i++ {
		s := strconv.Itoa(i)
		c.Set(s, s)
	}
	s := ""
	for k := c.head; k != ""; k = c.data[k].next {
		s = s + c.data[k].value
	}

	expected := "0123456789"
	if s != expected {
		t.Errorf("expected sequence %q, got %q", expected, s)
	}

	_ = c.Del("0")
	_ = c.Del("5")
	c.Set("7", "7")

	s = ""
	for k := c.head; k != ""; k = c.data[k].next {
		s = s + c.data[k].value
	}

	expected = "12346897"
	if s != expected {
		t.Errorf("expected sequence %q, got %q", expected, s)
	}
}

func TestTTLCleanup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := NewMapTTLCache[string, string](ctx, time.Millisecond, time.Millisecond*5)
	c.Set("key", "value")

	// Check we can get the value.
	val, err := c.Get("key")
	if err != nil {
		t.Errorf("unexpected error in Get: %v", err)
	}

	if val != "value" {
		t.Errorf("expected value %q, but got %v", "value", val)
	}

	time.Sleep(time.Millisecond * 10)

	// Check the value does not exist.
	if _, err := c.Get("key"); err != ErrNotFound {
		t.Errorf("expected error %v, but got %v", ErrNotFound, err)
	}

	// Cleanup should have purged the cache.
	if len(c.data) != 0 {
		t.Errorf("expected cache data len to be 0 but got %d", len(c.data))
	}
}

func TestTTLScenario(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := NewMapTTLCache[string, string](ctx, time.Millisecond, time.Millisecond*5)
	for i := 0; i < 10; i++ {
		s := strconv.Itoa(i)
		c.Set(s, s)
	}
	ts := time.Now()

	// Yep, accessing private variables in the test. So what?
	c.mux.Lock()
	c.now = func() time.Time {
		return ts.Add(time.Second)
	}
	c.mux.Unlock()

	for i := 0; i < 10; i++ {
		if i%2 == 0 {
			continue
		}
		s := strconv.Itoa(i)
		c.Set(s, s)
	}

	// Wait 10ms to be sure that cleanup goroutine runs
	// at least once even on slow CI runner.
	time.Sleep(time.Millisecond * 10)

	c.mux.Lock()
	// Half of the records should be removed by now.
	if len(c.data) != 5 {
		t.Errorf("expected cache data len to be 5 but got %d", len(c.data))
	}
	c.mux.Unlock()

	// Check we can get odd values, but not even.
	for i := 0; i < 10; i++ {
		s := strconv.Itoa(i)
		val, err := c.Get(s)
		if i%2 == 0 {
			if err != ErrNotFound {
				t.Errorf("expected to get %v but got %v", ErrNotFound, err)
			}
			continue
		}

		if err != nil {
			t.Errorf("unexpected error in Get: %v", err)
		}

		if val != s {
			t.Errorf("expected value %q, but got %v", s, val)
		}

	}

	time.Sleep(time.Millisecond * 5)

	// Cleanup should remove all even values.
	if len(c.data) != 5 {
		t.Errorf("expected cache data len to be 5 but got %d", len(c.data))
	}
}

func TestSetIfPresentResetsTTL(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := NewMapTTLCache[string, string](ctx, time.Second, time.Second)

	ts := time.Now()

	c.Set("key", "value")
	c.mux.Lock()
	c.now = func() time.Time { return ts.Add(time.Second) }
	c.mux.Unlock()

	if !c.SetIfPresent("key", "value2") {
		t.Errorf("expected key to be set as it is present in the map, but SetIfPresent returned false")
	}

	v, err := c.Get("key")
	if err != nil {
		t.Errorf("unexpected error in Get: %v", err)
	}

	if v != "value2" {
		t.Errorf("value was not updated by SetIfPresent, expected %v, but got %v", "value2", v)
	}
}
