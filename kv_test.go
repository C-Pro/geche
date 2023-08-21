package geche

import (
	"errors"
	"fmt"
	"math/rand"
	"runtime"
	"sort"
	"strings"
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

	// t.Log(got)
	if len(exp) != len(got) {
		t.Fatalf("expected length %d, got %d", len(exp), len(got))
	}

	for i := 0; i < len(exp); i++ {
		if exp[i] != got[i] {
			t.Errorf("%d: expected %q, got %q", i, exp[i], got[i])
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

func TestKVEmptyPrefixDiffLen(t *testing.T) {
	cache := NewMapCache[string, string]()
	kv := NewKV[string](cache)

	kv.Set("12345", "12345")
	kv.Set("123", "123")
	kv.Set("3", "3")
	kv.Set("2", "2")
	kv.Set("33333", "33333")
	kv.Set("1", "1")

	expected := []string{"1", "123", "12345", "2", "3", "33333"}

	got, err := kv.ListByPrefix("")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}

	compareSlice(t, expected, got)
}

func genRandomString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(rand.Intn(256))
	}
	return string(b)
}

func TestKVEmptyPrefixFuzz(t *testing.T) {
	cache := NewMapCache[string, string]()
	kv := NewKV[string](cache)

	set := map[string]struct{}{}
	for i := 0; i < 10000; i++ {
		key := genRandomString(rand.Intn(300) + 1)
		set[key] = struct{}{}
		kv.Set(key, key)
	}

	expected := []string{}
	for key := range set {
		expected = append(expected, key)
	}
	sort.Strings(expected)

	got, err := kv.ListByPrefix("")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}

	compareSlice(t, expected, got)
}

// This test creates 10k random KV pairs. Each key is prefixed with one of 10
// random prefixes. Then it deletes 10% of keys and checks that ListByPrefix
// returns correct results.
func TestKVPrefixFuzz(t *testing.T) {
	prefixes := []string{}
	for i := 0; i < 10; i++ {
		prefixes = append(prefixes, genRandomString(rand.Intn(20)+1))
	}
	cache := NewMapCache[string, string]()
	kv := NewKV[string](cache)

	set := map[string]struct{}{}
	for i := 0; i < 10000; i++ {
		prefix := prefixes[rand.Intn(len(prefixes))]
		pl := rand.Intn(len(prefix))
		key := prefix[:pl] + genRandomString(rand.Intn(300)+1)
		set[key] = struct{}{}
		kv.Set(key, key)
	}

	// Delete 10% of keys.
	for key := range set {
		if rand.Float64() < 0.1 {
			delete(set, key)
			_ = kv.Del(key)
		}
	}

	expected := []string{}
	for key := range set {
		expected = append(expected, key)
	}
	sort.Strings(expected)

	got, err := kv.ListByPrefix("")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}

	compareSlice(t, expected, got)

	for i := 1; i < len(prefixes); i++ {
		prefix := prefixes[i]
		for j := 1; j < len(prefix); j++ {
			q := prefix[:j]
			expected2 := make([]string, 0, len(expected))
			for _, key := range expected {
				if strings.HasPrefix(key, q) {
					expected2 = append(expected2, key)
				}
			}

			got, err := kv.ListByPrefix(q)
			if err != nil {
				t.Fatalf("unexpected error in ListByPrefix: %v", err)
			}

			compareSlice(t, expected2, got)
		}
	}
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

	if err := kv.Del("nonexistent"); err != nil {
		t.Errorf("unexpected error in Del: %v", err)
	}
}

func TestKVEmptyKEy(t *testing.T) {
	cache := NewMapCache[string, string]()
	kv := NewKV[string](cache)

	kv.Set("", "0")
	kv.Set("foo1", "1")
	kv.Set("foo2", "2")

	got, err := kv.ListByPrefix("fo")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}

	if len(got) > 3 {
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

func TestKVAlloc(t *testing.T) {
	cache := NewMapCache[string, string]()
	kv := NewKV[string](cache)

	var (
		mBefore, mAfter runtime.MemStats
		rawDataLen      int64
	)
	runtime.GC()
	runtime.ReadMemStats(&mBefore)

	for i := 0; i < 10000; i++ {
		key := genRandomString(rand.Intn(300) + 1)
		rawDataLen += int64(len(key) * 2)
		kv.Set(key, key)
	}

	runtime.GC()
	runtime.ReadMemStats(&mAfter)
	t.Logf("rawDataLen: %d", rawDataLen)
	t.Logf("memIncrease: %d", mAfter.HeapAlloc-mBefore.HeapAlloc)
	t.Logf("memIncreaseRatio: %d", int(float64(mAfter.HeapAlloc-mBefore.HeapAlloc)/float64(rawDataLen)))

	keys, err := kv.ListByPrefix("")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}

	for _, key := range keys {
		_ = kv.Del(key)
	}

	runtime.GC()
	runtime.ReadMemStats(&mAfter)
	t.Logf("memIncreaseAfterDel: %d", mAfter.HeapAlloc-mBefore.HeapAlloc)

	if mAfter.HeapAlloc > mBefore.HeapAlloc {
		if mAfter.HeapAlloc-mBefore.HeapAlloc > uint64(rawDataLen) {
			t.Errorf("memory increase is too big")
		}
	}

	if len(kv.trie.down) > 0 {
		t.Log(kv.trie.down)
		t.Errorf("trie is not empty")
	}
}

func TestKVDel(t *testing.T) {
	cache := NewMapCache[string, string]()
	kv := NewKV[string](cache)

	kv.Set("foo", "bar")
	_ = kv.Del("foo")

	if len(kv.trie.down) > 0 {
		t.Error("trie is not empty")
	}

	kv.Set("fo", "bar")
	kv.Set("food", "bar")
	_ = kv.Del("food")

	if len(kv.trie.down) != 1 {
		t.Errorf("expectedf root trie to have 1 element, got %d", len(kv.trie.down))
	}
}
