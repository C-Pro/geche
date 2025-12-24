// nolint:errcheck
package geche

import (
	"math/rand"
	"sync"
	"testing"
)

func TestLockerParallel(t *testing.T) {
	// Use locker to simulate atomic balance transfer between accounts.
	// Single transfer consists of getting balance of two accounts,
	// then subtracting some amount from one and adding it to another.
	// Operation is run concurrently on multiple goroutines,
	// If tx isolation is not implemented correctly total balance can change.
	locker := NewLocker[int, int](NewMapCache[int, int]())

	numAccounts := 10
	numTransactions := 100000
	initialBalance := 1000

	tx := locker.Lock()
	for i := 0; i < numAccounts; i++ {
		tx.Set(i, initialBalance)
	}
	tx.Unlock()
	totalBalance := numAccounts * initialBalance

	wg := &sync.WaitGroup{}
	for i := 0; i < numTransactions; i++ {
		wg.Add(1)
		go func() {
			accA := rand.Intn(numAccounts)
			var accB int
			for accB = rand.Intn(numAccounts); accB == accA; accB = rand.Intn(numAccounts) {
			}
			tx := locker.Lock()
			balA, _ := tx.Get(accA)
			balB, _ := tx.Get(accB)

			if balA < balB {
				size := rand.Intn(balB)
				balA += size
				balB -= size
				tx.Set(accA, balA)
				tx.Set(accB, balB)
			} else {
				size := rand.Intn(balA)
				balB += size
				balA -= size
				tx.Set(accA, balA)
				tx.Set(accB, balB)
			}

			tx.Unlock()
			wg.Done()
		}()
	}

	wg.Wait()
	tx = locker.RLock()
	sum := 0
	for i := 0; i < numAccounts; i++ {
		bal, _ := tx.Get(i)
		sum += bal
	}
	tx.Unlock()

	if sum != totalBalance {
		t.Errorf("expected %d, got %d", totalBalance, sum)
	}
}

func panics(f func()) (panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			panicked = true
		}
	}()

	f()
	return
}

func TestLockerRPanics(t *testing.T) {
	locker := NewLocker[int, int](NewMapCache[int, int]())
	tx := locker.RLock()
	if !panics(func() { tx.Set(1, 1) }) {
		t.Errorf("expected panic (Set on RLocked)")
	}

	if !panics(func() { tx.Del(1) }) {
		t.Errorf("expected panic (Delete on RLocked)")
	}

	if !panics(func() { tx.SetIfPresent(1, 1) }) {
		t.Errorf("expected panic (SetIfPresent on RLocked)")
	}

	if !panics(func() { tx.SetIfAbsent(1, 1) }) {
		t.Errorf("expected panic (SetIfAbsent on RLocked)")
	}

	tx.Unlock()
	if !panics(func() { tx.Unlock() }) {
		t.Errorf("expected panic (Unlock on already unlocked)")
	}

	if !panics(func() { tx.Set(1, 1) }) {
		t.Errorf("expected panic (Set on already unlocked)")
	}

	if !panics(func() { tx.Del(1) }) {
		t.Errorf("expected panic (Delete on already unlocked)")
	}

	if !panics(func() { tx.Get(1) }) {
		t.Errorf("expected panic (Get on already unlocked)")
	}

	if !panics(func() { tx.Len() }) {
		t.Errorf("expected panic (Len on already unlocked)")
	}

	if !panics(func() { tx.Snapshot() }) {
		t.Errorf("expected panic (Snapshot on already unlocked)")
	}

	if !panics(func() { tx.SetIfPresent(1, 1) }) {
		t.Errorf("expected panic (SetIfPresent on already unlocked)")
	}

	if !panics(func() { tx.SetIfAbsent(1, 1) }) {
		t.Errorf("expected panic (SetIfAbsent on already unlocked)")
	}
}

func TestLockerRWPanics(t *testing.T) {
	locker := NewLocker[int, int](NewMapCache[int, int]())
	tx := locker.Lock()

	if !panics(func() { _, _ = tx.ListByPrefix("test") }) {
		t.Errorf("expected panic (ListByPrefix on MapCache)")
	}

	tx.Unlock()
	if !panics(func() { tx.Unlock() }) {
		t.Errorf("expected panic (Unlock on already unlocked)")
	}

	if !panics(func() { tx.Set(1, 1) }) {
		t.Errorf("expected panic (Set on already unlocked)")
	}

	if !panics(func() { tx.Del(1) }) {
		t.Errorf("expected panic (Delete on already unlocked)")
	}

	if !panics(func() { tx.Get(1) }) {
		t.Errorf("expected panic (Get on already unlocked)")
	}

	if !panics(func() { tx.Len() }) {
		t.Errorf("expected panic (Len on already unlocked)")
	}

	if !panics(func() { tx.Snapshot() }) {
		t.Errorf("expected panic (Snapshot on already unlocked)")
	}

	if !panics(func() { _, _ = tx.ListByPrefix("test") }) {
		t.Errorf("expected panic (ListByPrefix on already unlocked)")
	}
}

func TestLockerListByPrefix(t *testing.T) {
	imp := NewLocker[string, string](NewKV[string](NewMapCache[string, string]()))
	tx := imp.Lock()
	defer tx.Unlock()

	tx.Set("test9", "test9")
	tx.Set("test2", "test2")
	tx.Set("test1", "test1")
	tx.Set("test3", "test3")

	_ = tx.Del("test2")

	expected := []string{"test1", "test3", "test9"}
	actual, err := tx.ListByPrefix("test")
	if err != nil {
		t.Errorf("unexpected error in ListByPrefix: %v", err)
	}

	compareSlice(t, expected, actual)
}

func TestLockerListByPrefixCache(t *testing.T) {
	imp := NewLocker(NewKVCache[string, string]())
	tx := imp.Lock()
	defer tx.Unlock()

	tx.Set("test9", "test9")
	tx.Set("test2", "test2")
	tx.Set("test1", "test1")
	tx.Set("test3", "test3")

	_ = tx.Del("test2")

	expected := []string{"test1", "test3", "test9"}
	actual, err := tx.ListByPrefix("test")
	if err != nil {
		t.Errorf("unexpected error in ListByPrefix: %v", err)
	}

	compareSlice(t, expected, actual)
}
