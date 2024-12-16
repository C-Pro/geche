package geche

import (
	"context"
	"math/rand"
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

func TestHeadTailLogicConcurrent(t *testing.T) {
	m := NewMapTTLCache[string, string](context.Background(), time.Millisecond, time.Hour)

	pool := make([]string, 50)
	for i := range pool {
		pool[i] = randomString(10)
	}

	for i := 0; i < 1000; i++ {
		idx := rand.Intn(len(pool))
		go func() {
			_, err := m.Get(pool[idx])
			if err != nil {
				m.Set(pool[idx], pool[idx])
			}
		}()
	}

	time.Sleep(time.Second)

	var expectedHeadKey, expectedTailKey string
	var expectedHeadValue, expectedTailValue ttlRec[string, string]

	keys := map[string]struct{}{}

	for k, v := range m.data {
		keys[k] = struct{}{}
		if expectedTailKey == "" {
			expectedTailKey = k
			expectedHeadKey = k
			expectedHeadValue = v
			expectedTailValue = v
		} else if expectedTailValue.timestamp.Before(v.timestamp) {
			expectedTailKey = k
			expectedTailValue = v
		} else if expectedHeadValue.timestamp.After(v.timestamp) {
			expectedHeadKey = k
			expectedHeadValue = v
		}
	}

	for k, v := range m.data {
		if _, ok := keys[v.next]; k != m.tail && !ok {
			t.Errorf("expected key %q not found in data", v.next)
		}

		if _, ok := keys[v.prev]; k != m.head && !ok {
			t.Errorf("expected key %q not found in data", v.prev)
		}
	}

	if m.head != expectedHeadKey {
		t.Errorf("expected head key %q, but got %v", expectedHeadKey, m.head)
	}

	if m.tail != expectedTailKey {
		t.Errorf("expected tail key %q, but got %v", expectedTailKey, m.tail)
	}

	if err := m.cleanup(); err != nil {
		t.Errorf("unexpected error in cleanup: %v", err)
	}

	if m.Len() != 0 {
		t.Errorf("expected clean to have %d elements, but got %d", 0, m.Len())
	}
}

func TestReinsertHead(t *testing.T) {
	c := NewMapTTLCache[string, string](context.Background(), time.Millisecond, time.Second)
	c.Set("k1", "v1")
	c.Set("k2", "v2")
	c.Set("k3", "v3")
	c.Set("k1", "v2")
	time.Sleep(2 * time.Millisecond)
	if err := c.cleanup(); err != nil {
		t.Errorf("unexpected cleanup error: %v", err)
	}

	if c.Len() != 0 {
		t.Errorf("expected cache data len to be 0 but got %d", c.Len())
	}
}

func TestReinsertTail(t *testing.T) {
	c := NewMapTTLCache[string, string](context.Background(), time.Millisecond, time.Second)
	c.Set("k1", "v1")
	c.Set("k2", "v2")
	c.Set("k3", "v3")
	c.Set("k3", "v4")
	time.Sleep(2 * time.Millisecond)

	if c.data["k3"].next != "" {
		t.Errorf("expected tail next to be zero")
	}

	if c.data["k3"].prev != "k2" {
		t.Errorf("expected tail prev to be k2, but got %s", c.data["k3"].prev)
	}

	if err := c.cleanup(); err != nil {
		t.Errorf("unexpected cleanup error: %v", err)
	}

	if c.Len() != 0 {
		t.Errorf("expected cache data len to be 0 but got %d", c.Len())
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

	old, inserted := c.SetIfPresent("key", "value2")
	if !inserted {
		t.Errorf("expected key to be set as it is present in the map, but SetIfPresent returned false")
	}

	if old != "value" {
		t.Errorf("expected old value %q, but got %q", "value", old)
	}

	v, err := c.Get("key")
	if err != nil {
		t.Errorf("unexpected error in Get: %v", err)
	}

	if v != "value2" {
		t.Errorf("value was not updated by SetIfPresent, expected %v, but got %v", "value2", v)
	}
}
