package geche

import (
	"errors"
	"fmt"
	"math/rand"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
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

	if len(exp) != len(got) {
		t.Logf("expect: %v", exp)
		t.Logf("got: %v", got)
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
		if key == "008" {
			kv.Set(key, key)
		}
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
		b[i] = byte(rand.Intn(26) + int(byte('a')))
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

func TestKVListByPrefix2Error(t *testing.T) {
	cache := &MockErrCache{}
	kv := NewKV[string](cache)

	kv.Set("e", "wat")
	kv.Set("er", "wat")
	kv.Set("err", "something")
	_, err := kv.ListByPrefix("err")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

// To check that the error is propagated correctly.
type MockErrCache struct{}

func (m *MockErrCache) Set(key string, value string)                         {}
func (m *MockErrCache) SetIfPresent(key string, value string) (string, bool) { return "", false }
func (m *MockErrCache) Del(key string) error                                 { return nil }
func (m *MockErrCache) Snapshot() map[string]string                          { return nil }
func (m *MockErrCache) Len() int                                             { return 0 }
func (m *MockErrCache) Get(key string) (string, error) {
	if key == "err" {
		return "", errors.New("wow an error")
	}
	return "", nil
}

func TestKVAlloc(t *testing.T) {
	cache := NewMapCache[string, string]()
	kv := NewKV(cache)

	var (
		mBefore, mAfter runtime.MemStats
		rawDataLen      int64
	)
	runtime.GC()
	runtime.ReadMemStats(&mBefore)

	for i := 0; i < 1_000_000; i++ {
		key := genRandomString(rand.Intn(300) + 1)
		rawDataLen += int64(len(key) * 2)

		kv.Set(key, key)
	}

	runtime.GC()
	runtime.ReadMemStats(&mAfter)
	t.Logf("rawDataLen: %d", rawDataLen)
	t.Logf("memIncrease: %d", mAfter.HeapAlloc-mBefore.HeapAlloc)
	t.Logf("memIncreaseRatio: %0.1f", float64(mAfter.HeapAlloc-mBefore.HeapAlloc)/float64(rawDataLen))

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

	if len(kv.trie.children) > 0 {
		t.Log(kv.trie.children)
		t.Errorf("trie is not empty")
	}

	if kv.Len() > 0 {
		t.Errorf("data is not empty")
	}
}

func TestKVDel(t *testing.T) {
	cache := NewMapCache[string, string]()
	kv := NewKV[string](cache)

	kv.Set("foo", "bar")
	_ = kv.Del("foo")

	if _, err := kv.Get("foo"); err == nil {
		t.Error("expected error after deleting a key, got nil")
	}

	kv.Set("fo", "bar")
	kv.Set("food", "bar")
	_ = kv.Del("food")

	res, err := kv.ListByPrefix("foo")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}
	if len(res) != 0 {
		t.Errorf("expected 0 results, got %d", len(res))
	}

	res, err = kv.ListByPrefix("fo")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}
	if len(res) != 1 {
		t.Errorf("expected 1 results, got %d", len(res))
	}
}

func TestCommonPrefixLen(t *testing.T) {
	cases := []struct {
		a, b string
		exp  int
	}{
		{"", "", 0},
		{"a", "", 0},
		{"", "a", 0},
		{"a", "a", 1},
		{"a", "b", 0},
		{"ab", "a", 1},
		{"a", "ab", 1},
		{"ab", "ab", 2},
		{"ab", "ac", 1},
		{"ab", "abc", 2},
		{"ab", "abc", 2},
	}

	for _, tc := range cases {
		got := commonPrefixLen([]byte(tc.a), []byte(tc.b))
		if got != tc.exp {
			t.Errorf("expected %d, got %d", tc.exp, got)
		}
	}
}

func TestSetEmptyKey(t *testing.T) {
	kv := NewKV[string](NewMapCache[string, string]())
	kv.Set("", "test")

	got, err := kv.Get("")
	if err != nil {
		t.Fatalf("unexpected error in Get: %v", err)
	}
	if got != "test" {
		t.Errorf("expected %q, got %q", "test", got)
	}

	values, err := kv.ListByPrefix("")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}
	if len(values) != 1 {
		t.Errorf("expected len %d, got %d", 1, len(values))
	}
	if values[0] != "test" {
		t.Errorf("expected %q, got %q", "test", values[0])
	}
}

func TestTwoSingleChar(t *testing.T) {
	kv := NewKV[string](NewMapCache[string, string]())
	kv.Set("a", "test1")
	kv.Set("b", "test2")

	values, err := kv.ListByPrefix("")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}
	expected := []string{"test1", "test2"}
	compareSlice(t, expected, values)
}

func TestSetTwoDepth(t *testing.T) {
	kv := NewKV[string](NewMapCache[string, string]())
	kv.Set("a", "test1")
	kv.Set("ab", "test2")

	values, err := kv.ListByPrefix("")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}
	expected := []string{"test1", "test2"}
	compareSlice(t, expected, values)
}

func TestSetTwoDepthReverseOrder(t *testing.T) {
	kv := NewKV[string](NewMapCache[string, string]())
	// When the order of Set is reversed, first Set will add 2-symbol node,
	// and second set will split it into two.
	kv.Set("ab", "test2")
	kv.Set("a", "test1")

	values, err := kv.ListByPrefix("")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}
	expected := []string{"test1", "test2"}
	compareSlice(t, expected, values)
}

func TestSetAppendTail(t *testing.T) {
	kv := NewKV[string](NewMapCache[string, string]())
	kv.Set("ab", "test2")
	kv.Set("abc", "test1")

	values, err := kv.ListByPrefix("")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}
	expected := []string{"test2", "test1"}
	compareSlice(t, expected, values)
}

func TestSet3(t *testing.T) {
	// Some tests like this here are white-box ones to cover specific code paths,
	// or to check for regressions of fixed issues found by fuzzing.
	kv := NewKV[string](NewMapCache[string, string]())
	kv.Set("ab", "test2")
	kv.Set("abc", "test1")
	kv.Set("a", "test4")

	values, err := kv.ListByPrefix("")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}

	expected := []string{
		"test4", "test2", "test1",
	}

	compareSlice(t, expected, values)
}

func TestSet4(t *testing.T) {
	kv := NewKV[string](NewMapCache[string, string]())
	kv.Set("ab", "test2")
	kv.Set("abc", "test1")
	kv.Set("abz", "test4")

	values, err := kv.ListByPrefix("")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}

	expected := []string{
		"test2", "test1", "test4",
	}

	t.Log(values)
	compareSlice(t, expected, values)
}

func TestSet5(t *testing.T) {
	kv := NewKV[string](NewMapCache[string, string]())
	kv.Set("abra", "test2")
	kv.Set("cadabra", "test1")
	kv.Set("abracadabra", "test4")

	values, err := kv.ListByPrefix("cad")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}

	expected := []string{
		"test1",
	}

	t.Log(values)
	compareSlice(t, expected, values)
}

func TestSetIfPresent(t *testing.T) {
	kv := NewKV[string](NewMapCache[string, string]())
	kv.Set("a", "test2")
	kv.Set("b", "test1")
	kv.Set("c", "test4")

	old, inserted := kv.SetIfPresent("a", "test5")
	if !inserted {
		t.Errorf("key \"a\" is present in kv, SetIfPresent should return true")
	}

	if old != "test2" {
		t.Errorf("old value is %q, SetIfPresent should return true", old)
	}

	old, inserted = kv.SetIfPresent("a", "test6")
	if !inserted {
		t.Errorf("key \"a\" is present in kv, SetIfPresent should return true")
	}

	if old != "test5" {
		t.Errorf("old value associated with \"a\" is not \"test5\"")
	}

	if _, inserted := kv.SetIfPresent("d", "test3"); inserted {
		t.Errorf("key \"bbb\" is not present in kv, SetIfPresent should return false")
	}

	if _, inserted := kv.SetIfPresent("d", "test3"); inserted {
		t.Errorf("key \"d\" is still not present in kv, SetIfPresent should return false")
	}

	val, err := kv.Get("a")
	if err != nil {
		t.Fatalf("unexpected error in Get: %v", err)
	}

	if val != "test6" {
		t.Errorf("expected %q, got %q", "test6", val)
	}

	_, err = kv.Get("d")
	if err == nil {
		t.Errorf("expected key \"d\" to not be present in the kv")
	}
}

func TestSetIfPresentConcurrent(t *testing.T) {
	kv := NewKV[string](NewMapCache[string, string]())
	kv.Set("a", "startA")
	kv.Set("b", "startB")

	for i := 0; i < 1000; i++ {
		go func() {
			switch rand.Intn(6) {
			case 0:
				kv.SetIfPresent("a", "a")
			case 1:
				kv.SetIfPresent("b", "b")
			case 2:
				kv.SetIfPresent("c", "c")
			case 3:
				_, _ = kv.Get("a")
			case 4:
				_, _ = kv.Get("b")
			case 5:
				_, _ = kv.Get("c")
			}
		}()
	}

	time.Sleep(10 * time.Millisecond)

	if val, _ := kv.Get("a"); val != "a" {
		t.Errorf("expected %q, got %q", "a", val)
	}

	if val, _ := kv.Get("b"); val != "b" {
		t.Errorf("expected %q, got %q", "b", val)
	}

	if _, err := kv.Get("c"); err == nil {
		t.Errorf("expected key \"c\" to not be present in the kv")
	}
}

func FuzzKVSetListByPrefix(f *testing.F) {
	// Simple fuzzing test adding 3 keys then listing by prefix.
	examples := [][]string{
		{"", "", "", ""},
		{"a", "a", "a", ""},
		{"a", "a", "a", "b"},
		{"ab", "ac", "abc", "a"},
		{"abra", "cadabra", "abracadabra", "cad"},
		{"abra", "cadabra", "abracadabra", "ab"},
		{"abcd", "abz", "ac", "a"},
		{"a", "abc", "abcd", "a"},
	}

	for _, example := range examples {
		f.Add(example[0], example[1], example[2], example[3])
	}

	f.Fuzz(func(t *testing.T, k1, k2, k3, prefix string) {
		golden := []string{
			k1, k2, k3,
		}
		kv := NewKV[string](NewMapCache[string, string]())
		for _, key := range golden {
			kv.Set(key, key)
		}

		sort.Strings(golden)

		expect := make([]string, 0, len(golden))

		seen := map[string]struct{}{}
		for _, s := range golden {
			if _, ok := seen[s]; !ok && strings.HasPrefix(s, prefix) {
				expect = append(expect, s)
				seen[s] = struct{}{}
			}
		}

		got, err := kv.ListByPrefix(prefix)
		if err != nil {
			t.Fatalf("unexpected error in ListByPrefix: %v", err)
		}

		t.Logf("params: (%q, %q, %q, %q)", k1, k2, k3, prefix)
		t.Logf("got: %v", got)

		compareSlice(t, expect, got)
	})
}

func TestKVDelNoprefix(t *testing.T) {
	kv := NewKV[string](NewMapCache[string, string]())
	kv.Set("hu", "hu")
	_ = kv.Del("h")
	res, err := kv.Get("hu")
	if err != nil {
		t.Errorf("unexpected error in Get: %v", err)
	}
	if res != "hu" {
		t.Errorf("expected %q, got %q", "hu", res)
	}
	l, err := kv.ListByPrefix("")
	if err != nil {
		t.Errorf("unexpected error in ListByPrefix: %v", err)
	}
	if len(l) != 1 {
		t.Fatalf("expected len 1, got %d", len(l))
	}

	if l[0] != "hu" {
		t.Errorf("expected %q, got %q", "hu", res)
	}
}

func FuzzMonkey(f *testing.F) {
	// More elaborate fuzzing test. It creates a random task of 50 Set/Del
	// commands to be executed on a KV. Then it checks that ListByPrefix
	// returns correct results.
	examples := []struct {
		seed   int64
		prefix string
	}{
		{0, ""},
		{439, "x"},
		{2, "ab"},
		{4928589, " "},
		{93, "1"},
		{1994, ""},
		{185, "P"},
	}
	for _, example := range examples {
		f.Add(example.seed, example.prefix)
	}

	f.Fuzz(func(t *testing.T, seed int64, prefix string) {
		kv := NewKV[string](NewMapCache[string, string]())
		task := randTask(seed)
		golden := make(map[string]struct{}, len(task))
		for _, cmd := range task {
			switch cmd.action {
			case "Set":
				kv.Set(cmd.key, cmd.key)
				golden[cmd.key] = struct{}{}
			case "Del":
				// Since keys are random we expect a lot of Del to fail.
				_ = kv.Del(cmd.key)
				delete(golden, cmd.key)
			}
		}

		goldenFiltered := make([]string, 0, len(golden))
		for k := range golden {
			if strings.HasPrefix(k, prefix) {
				goldenFiltered = append(goldenFiltered, k)
			}
		}
		sort.Strings(goldenFiltered)

		got, err := kv.ListByPrefix(prefix)
		if err != nil {
			t.Fatalf("unexpected error in ListByPrefix: %v", err)
		}

		if kv.Len() != len(golden) {
			t.Errorf("expected len %d, got %d", len(golden), kv.Len())
		}

		for _, key := range goldenFiltered {
			val, err := kv.Get(key)
			if err != nil {
				t.Fatalf("unexpected error in Get: %v", err)
			}
			if val != key {
				t.Errorf("expected %q, got %q", key, val)
			}
		}

		t.Logf("seed: %d, task %v, prefix: %q", seed, task, prefix)
		compareSlice(t, goldenFiltered, got)
	})
}

type command struct {
	action string
	key    string
}

const (
	taskSize      = 50
	taskMinKeyLen = 1
	taskMaxKeyLen = 5
)

func randTask(seed int64) []command {
	task := make([]command, taskSize)
	r := rand.New(rand.NewSource(seed))

	for i := 0; i < len(task); i++ {
		task[i].action = "Set"
		if r.Float64() < 0.1 {
			task[i].action = "Del"
		}
		task[i].key = genRandomString(
			r.Intn(taskMaxKeyLen-taskMinKeyLen) +
				taskMinKeyLen,
		)
	}

	return task
}
