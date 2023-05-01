package geche

import (
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"
)

var theError = errors.New("OOPS! Somebody has dropped the production database!")

func updateFn(key string) (string, error) {
	return key, nil
}

func updateErrFn(key string) (string, error) {
	return "", theError
}

func TestUpdaterScenario(t *testing.T) {
	u := NewCacheUpdater[string, string](
		NewMapCache[string, string](),
		updateFn,
		2,
	)

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
}

func TestUpdaterErr(t *testing.T) {
	u := NewCacheUpdater[string, string](
		NewMapCache[string, string](),
		updateErrFn,
		2,
	)

	_, err := u.Get("test")
	if err != theError {
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
