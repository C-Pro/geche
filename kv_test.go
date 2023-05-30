package geche

import (
	"errors"
	"fmt"
	"testing"
)

func ExampleNewKV() {
	cache := NewMapCache[string, string]()
	kv := NewKV[string](cache)

	kv.Set("foo", "bar")
	kv.Set("foo2", "bar2")
	kv.Set("foo3", "bar3")
	kv.Set("foo1", "bar1")

	res, _ := kv.ListByPrefix("foo")
	fmt.Println(res)
	// Output: [bar bar1 bar2 bar3]
}

func compareSlice(t *testing.T, exp, got []string) {
	t.Helper()

	t.Log(got)
	if len(exp) != len(got) {
		t.Fatalf("expected length %d, got %d", len(exp), len(got))
	}

	for i := 0; i < len(exp); i++ {
		if exp[i] != got[i] {
			t.Errorf("expected %q, got %q", exp[i], got[i])
		}
	}
}

func TestKV(t *testing.T) {
	cache := NewMapCache[string, string]()
	kv := NewKV[string](cache)

	for i := 999; i >= 0; i-- {
		key := fmt.Sprintf("%03d", i)
		kv.Set(key, key)
	}

	expected := []string{
		"000", "001", "002", "003", "004", "005", "006", "007", "008", "009",
	}

	got, err := kv.ListByPrefix("00")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}
	compareSlice(t, expected, got)

	expected = []string{
		"120", "121", "122", "123", "124", "125", "126", "127", "128", "129",
	}

	got, err = kv.ListByPrefix("12")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}
	compareSlice(t, expected, got)

	expected = []string{"888"}

	got, err = kv.ListByPrefix("888")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}
	compareSlice(t, expected, got)

	_ = kv.Del("777")
	_ = kv.Del("779")

	if _, err := kv.Get("777"); err != ErrNotFound {
		t.Fatalf("expected error %v, got %v", ErrNotFound, err)
	}

	expected = []string{
		"770", "771", "772", "773", "774", "775", "776", "778",
	}

	got, err = kv.ListByPrefix("77")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}

	kv.Set("777", "777")
	kv.Set("779", "779")

	compareSlice(t, expected, got)

	expected = []string{
		"770", "771", "772", "773", "774", "775", "776", "777", "778", "779",
	}

	got, err = kv.ListByPrefix("77")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}

	compareSlice(t, expected, got)

	kv.Set("77", "77")

	expected = []string{
		"77", "770", "771", "772", "773", "774", "775", "776", "777", "778", "779",
	}

	got, err = kv.ListByPrefix("77")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}

	compareSlice(t, expected, got)
}

func TestKVEmptyPrefix(t *testing.T) {
	cache := NewMapCache[string, string]()
	kv := NewKV[string](cache)

	expected := []string{}
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("%02d", i)
		expected = append(expected, key)
		kv.Set(key, key)
	}

	got, err := kv.ListByPrefix("")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}

	compareSlice(t, expected, got)
}

func TestKVNonexist(t *testing.T) {
	cache := NewMapCache[string, string]()
	kv := NewKV[string](cache)

	kv.Set("test", "best")

	got, err := kv.ListByPrefix("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}

	if len(got) > 0 {
		t.Errorf("unexpected len %d", len(got))
	}
}

func TestKVError(t *testing.T) {
	cache := &MockErrCache{}
	kv := NewKV[string](cache)

	kv.Set("err", "something")
	_, err := kv.ListByPrefix("err")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	kv.Set("er", "another one")
	_, err = kv.ListByPrefix("e")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

// To check that the error is propagated correctly.
type MockErrCache struct{}

func (m *MockErrCache) Set(key string, value string) {}
func (m *MockErrCache) Del(key string) error         { return nil }
func (m *MockErrCache) Snapshot() map[string]string  { return nil }
func (m *MockErrCache) Len() int                     { return 0 }
func (m *MockErrCache) Get(key string) (string, error) {
	if key == "err" {
		return "", errors.New("wow an error")
	}
	return "", nil
}
