package geche

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"
)

var errThe = errors.New("OOPS! Somebody has dropped the production database")

func updateFn(key string) (string, error) {
	return key, nil
}

func updateErrFn(key string) (string, error) {
	return "", errThe
}

func ExampleNewCacheUpdater() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a cache updater with updaterFunction
	// that sets cache value equal to the key.
	u := NewCacheUpdater[string, string](
		NewMapTTLCache[string, string](ctx, time.Minute, time.Minute),
		updateFn,
		2,
	)

	// Value is not in the cache yet.
	// But it will be set by the updater function.
	v, _ := u.Get("nonexistent")
	fmt.Println(v)

	// Output: nonexistent
}

func TestUpdaterScenario(t *testing.T) {
	u := NewCacheUpdater[string, string](
		NewMapCache[string, string](),
		updateFn,
		2,
	)

	if u.Len() != 0 {
		t.Errorf("expected length to be 0, but got %d", u.Len())
	}

	v1, err := u.Get("test")
	if err != nil {
		t.Errorf("unexpected error in Get: %v", err)
	}

	v2, err := u.Get("test")
	if err != nil {
		t.Errorf("unexpected error in Get: %v", err)
	}

	if v1 != "test" || v1 != v2 {
		t.Errorf("expected both values to be %q, but got %q, %q", "test", v1, v2)
	}

	if u.Len() != 1 {
		t.Errorf("expected length to be 1, but got %d", u.Len())
	}

	if err := u.Del("test"); err != nil {
		t.Errorf("unexpected error in del: %v", err)
	}

	v, err := u.Get("test")
	if err != nil {
		t.Errorf("unexpected error in Get: %v", err)
	}

	if v != "test" {
		t.Errorf("expected to get %q, but got %q", "test", v)
	}

	u.Set("test", "best")

	v3, err := u.Get("test")
	if err != nil {
		t.Errorf("unexpected error in Get: %v", err)
	}

	if v3 != "best" {
		t.Errorf("expected to get %q, but got %q", "best", v3)
	}

	s := u.Snapshot()
	if len(s) != 1 {
		t.Errorf("expected snapshot length to be 1, but got %d", len(s))
	}

	if s["test"] != "best" {
		t.Errorf("expected to get %q, but got %q", "best", s["test"])
	}
}

func TestUpdaterSetIfPresent(t *testing.T) {
	u := NewCacheUpdater[string, string](
		NewMapCache[string, string](),
		updateFn,
		2,
	)

	if u.Len() != 0 {
		t.Errorf("expected length to be 0, but got %d", u.Len())
	}

	s, inserted := u.SetIfPresent("test", "test")
	if inserted {
		t.Error("expected not to insert the value")
	}

	if s != "" {
		t.Errorf("expected to get empty string, but got %q", s)
	}

	u.Set("test", "test")

	s, inserted = u.SetIfPresent("test", "test2")
	if !inserted {
		t.Error("expected to insert the value")
	}

	if s != "test" {
		t.Errorf("expected to get %q, but got %q", "test", s)
	}

	v, err := u.Get("test")
	if err != nil {
		t.Errorf("unexpected error in Get: %v", err)
	}

	if v != "test2" {
		t.Errorf("expected to get %q, but got %q", "test2", v)
	}
}

func TestUpdaterErr(t *testing.T) {
	u := NewCacheUpdater[string, string](
		NewMapCache[string, string](),
		updateErrFn,
		2,
	)

	_, err := u.Get("test")
	if err != errThe {
		t.Errorf("expected to get theError, but got %v", err)
	}

	u.Set("test", "test")

	v, err := u.Get("test")
	if err != nil {
		t.Errorf("unexpected error in Get: %v", err)
	}

	if v != "test" {
		t.Errorf("expected to get %q, but got %q", "test", v)
	}
}

func TestUpdaterConcurrent(t *testing.T) {
	var (
		mux     sync.Mutex
		currCnt int
		peakCnt int
	)

	updateFn := func(key string) (string, error) {
		mux.Lock()
		currCnt++
		if currCnt > peakCnt {
			peakCnt = currCnt
		}
		mux.Unlock()

		defer func() {
			mux.Lock()
			currCnt--
			mux.Unlock()
		}()

		time.Sleep(time.Millisecond)
		return key, nil
	}

	poolSize := 4

	u := NewCacheUpdater[string, string](
		NewMapCache[string, string](),
		updateFn,
		poolSize,
	)

	wg := sync.WaitGroup{}
	wg.Add(1000)
	for i := 0; i < 1000; i++ {
		go func() {
			defer wg.Done()
			v, err := u.Get("test")
			if err != nil {
				t.Errorf("unexpected error in Get: %v", err)
			}

			if v != "test" {
				t.Errorf("expected to get %q, but got %q", "test", v)
			}
		}()
	}

	wg.Wait()

	if currCnt != 0 {
		t.Errorf("expected currCnt to be 0, got %d", currCnt)
	}

	// We are using one key, so peakCnt s(hould be 1.
	if peakCnt != 1 {
		t.Errorf("expected peakCnt to be %d, got %d", 1, peakCnt)
	}

	wg = sync.WaitGroup{}
	wg.Add(1000)
	for i := 0; i < 1000; i++ {
		go func(i int) {
			defer wg.Done()
			key := strconv.Itoa(i % 100)
			v, err := u.Get(key)
			if err != nil {
				t.Errorf("unexpected error in Get: %v", err)
			}

			if v != key {
				t.Errorf("expected to get %q, but got %q", key, v)
			}
		}(i)
	}

	wg.Wait()

	if currCnt != 0 {
		t.Errorf("expected currCnt to be 0, got %d", currCnt)
	}

	// For 100 distinct keys peakCnt should be limited to poolSize.
	if peakCnt != poolSize {
		t.Errorf("expected peakCnt to be %d, got %d", poolSize, peakCnt)
	}
}

func TestUpdaterListByPrefix(t *testing.T) {
	imp := NewCacheUpdater[string, string](NewKV[string](NewMapCache[string, string]()), updateFn, 2)

	imp.Set("test9", "test9")
	imp.Set("test2", "test2")
	imp.Set("test1", "test1")
	imp.Set("test3", "test3")

	_ = imp.Del("test2")

	expected := []string{"test1", "test3", "test9"}
	actual, err := imp.ListByPrefix("test")
	if err != nil {
		t.Errorf("unexpected error in ListByPrefix: %v", err)
	}

	compareSlice(t, expected, actual)
}

func TestUpdaterListByPrefixUnsupported(t *testing.T) {
	imp := NewCacheUpdater[string, string](NewMapCache[string, string](), updateFn, 2)

	if !panics(func() { _, _ = imp.ListByPrefix("test") }) {
		t.Error("ListByPrefix expected to panic if underlying cache does not provide ListByPrefix")
	}
}
