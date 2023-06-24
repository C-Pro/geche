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
}

func TestLockerRWPanics(t *testing.T) {
	locker := NewLocker[int, int](NewMapCache[int, int]())
	tx := locker.Lock()

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
}
