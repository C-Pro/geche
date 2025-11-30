package geche

import (
	"fmt"
	"math/rand"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

func ExampleNewKVCache() {
	cache := NewKVCache[string, string]()

	cache.Set("foo", "bar")
	cache.Set("foo2", "bar2")
	cache.Set("foo3", "bar3")
	cache.Set("foo1", "bar1")

	res, _ := cache.ListByPrefix("foo")
	fmt.Println(res)
	// Output: [bar bar1 bar2 bar3]
}

func TestKVCache(t *testing.T) {
	cache := NewKVCache[string, string]()

	for i := 999; i >= 0; i-- {
		key := fmt.Sprintf("%03d", i)
		if key == "008" {
			cache.Set(key, key)
		}
		cache.Set(key, key)
	}

	expected := []string{
		"000", "001", "002", "003", "004", "005", "006", "007", "008", "009",
	}

	got, err := cache.ListByPrefix("00")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}
	compareSlice(t, expected, got)

	expected = []string{
		"120", "121", "122", "123", "124", "125", "126", "127", "128", "129",
	}

	got, err = cache.ListByPrefix("12")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}
	compareSlice(t, expected, got)

	expected = []string{"888"}

	got, err = cache.ListByPrefix("888")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}
	compareSlice(t, expected, got)

	_ = cache.Del("777")
	_ = cache.Del("779")

	if _, err := cache.Get("777"); err != ErrNotFound {
		t.Fatalf("expected error %v, got %v", ErrNotFound, err)
	}

	expected = []string{
		"770", "771", "772", "773", "774", "775", "776", "778",
	}

	got, err = cache.ListByPrefix("77")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}

	cache.Set("777", "777")
	cache.Set("779", "779")

	compareSlice(t, expected, got)

	expected = []string{
		"770", "771", "772", "773", "774", "775", "776", "777", "778", "779",
	}

	got, err = cache.ListByPrefix("77")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}

	compareSlice(t, expected, got)

	cache.Set("77", "77")

	expected = []string{
		"77", "770", "771", "772", "773", "774", "775", "776", "777", "778", "779",
	}

	got, err = cache.ListByPrefix("77")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}

	compareSlice(t, expected, got)
}

func TestKVCacheEmptyPrefix(t *testing.T) {
	cache := NewKVCache[string, string]()

	expected := []string{}
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("%02d", i)
		expected = append(expected, key)
		cache.Set(key, key)
	}

	got, err := cache.ListByPrefix("")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}

	compareSlice(t, expected, got)
}

func TestKVCacheEmptyPrefixDiffLen(t *testing.T) {
	cache := NewKVCache[string, string]()

	cache.Set("12345", "12345")
	cache.Set("123", "123")
	cache.Set("3", "3")
	cache.Set("2", "2")
	cache.Set("33333", "33333")
	cache.Set("1", "1")

	expected := []string{"1", "123", "12345", "2", "3", "33333"}

	got, err := cache.ListByPrefix("")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}

	compareSlice(t, expected, got)
}

func TestKVCacheEmptyPrefixFuzz(t *testing.T) {
	cache := NewKVCache[string, string]()

	set := map[string]struct{}{}
	for i := 0; i < 10000; i++ {
		key := genRandomString(rand.Intn(300) + 1)
		set[key] = struct{}{}
		cache.Set(key, key)
	}

	expected := []string{}
	for key := range set {
		expected = append(expected, key)
	}
	sort.Strings(expected)

	got, err := cache.ListByPrefix("")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}

	compareSlice(t, expected, got)
}

func TestKVCachePrefixFuzz(t *testing.T) {
	prefixes := []string{}
	for i := 0; i < 10; i++ {
		prefixes = append(prefixes, genRandomString(rand.Intn(20)+1))
	}
	cache := NewKVCache[string, string]()

	set := map[string]struct{}{}
	for i := 0; i < 10000; i++ {
		prefix := prefixes[rand.Intn(len(prefixes))]
		pl := rand.Intn(len(prefix))
		key := prefix[:pl] + genRandomString(rand.Intn(300)+1)
		set[key] = struct{}{}
		cache.Set(key, key)
	}

	// Delete 10% of keys.
	for key := range set {
		if rand.Float64() < 0.1 {
			delete(set, key)
			_ = cache.Del(key)
		}
	}

	expected := []string{}
	for key := range set {
		expected = append(expected, key)
	}
	sort.Strings(expected)

	got, err := cache.ListByPrefix("")
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

			got, err := cache.ListByPrefix(q)
			if err != nil {
				t.Fatalf("unexpected error in ListByPrefix: %v", err)
			}

			compareSlice(t, expected2, got)
		}
	}
}

func TestKVCacheNonexist(t *testing.T) {
	cache := NewKVCache[string, string]()

	cache.Set("test", "best")

	got, err := cache.ListByPrefix("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}

	if len(got) > 0 {
		t.Errorf("unexpected len %d", len(got))
	}

	if err := cache.Del("nonexistent"); err != nil {
		t.Errorf("unexpected error in Del: %v", err)
	}
}

func TestKVCacheEmptyKey(t *testing.T) {
	cache := NewKVCache[string, string]()

	cache.Set("", "0")
	cache.Set("foo1", "1")
	cache.Set("foo2", "2")

	got, err := cache.ListByPrefix("fo")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}

	if len(got) > 3 {
		t.Errorf("unexpected len %d", len(got))
	}
}

func TestKVCacheAlloc(t *testing.T) {
	cache := NewKVCache[string, string]()

	var (
		mBefore, mAfter runtime.MemStats
		rawDataLen      int64
	)
	runtime.GC()
	runtime.ReadMemStats(&mBefore)

	for i := 0; i < 1_000_000; i++ {
		key := genRandomString(rand.Intn(300) + 1)
		rawDataLen += int64(len(key) * 2)

		cache.Set(key, key)
	}

	runtime.GC()
	runtime.ReadMemStats(&mAfter)
	t.Logf("rawDataLen: %d", rawDataLen)
	t.Logf("memIncrease: %d", mAfter.HeapAlloc-mBefore.HeapAlloc)
	t.Logf("memIncreaseRatio: %0.1f", float64(mAfter.HeapAlloc-mBefore.HeapAlloc)/float64(rawDataLen))

	keys, err := cache.ListByPrefix("")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}

	for _, key := range keys {
		_ = cache.Del(key)
	}

	runtime.GC()
	runtime.ReadMemStats(&mAfter)
	t.Logf("memIncreaseAfterDel: %d", mAfter.HeapAlloc-mBefore.HeapAlloc)

	if mAfter.HeapAlloc > mBefore.HeapAlloc {
		if mAfter.HeapAlloc-mBefore.HeapAlloc > uint64(rawDataLen) {
			t.Errorf("memory increase is too big")
		}
	}

	if len(cache.trie.children) > 0 {
		t.Log(cache.trie.children)
		t.Errorf("trie is not empty")
	}

	if cache.Len() > 0 {
		t.Errorf("data is not empty")
	}
}

func TestKVCacheDel(t *testing.T) {
	cache := NewKVCache[string, string]()

	cache.Set("foo", "bar")
	_ = cache.Del("foo")

	if _, err := cache.Get("foo"); err == nil {
		t.Error("expected error after deleting a key, got nil")
	}

	cache.Set("fo", "bar")
	cache.Set("food", "bar")
	_ = cache.Del("food")

	res, err := cache.ListByPrefix("foo")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}
	if len(res) != 0 {
		t.Errorf("expected 0 results, got %d", len(res))
	}

	res, err = cache.ListByPrefix("fo")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}
	if len(res) != 1 {
		t.Errorf("expected 1 results, got %d", len(res))
	}
}

func TestKVCacheSetEmptyKey(t *testing.T) {
	cache := NewKVCache[string, string]()
	cache.Set("", "test")

	got, err := cache.Get("")
	if err != nil {
		t.Fatalf("unexpected error in Get: %v", err)
	}
	if got != "test" {
		t.Errorf("expected %q, got %q", "test", got)
	}

	values, err := cache.ListByPrefix("")
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

func TestKVCacheTwoSingleChar(t *testing.T) {
	cache := NewKVCache[string, string]()
	cache.Set("a", "test1")
	cache.Set("b", "test2")

	values, err := cache.ListByPrefix("")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}
	expected := []string{"test1", "test2"}
	compareSlice(t, expected, values)
}

func TestKVCacheSetTwoDepth(t *testing.T) {
	cache := NewKVCache[string, string]()
	cache.Set("a", "test1")
	cache.Set("ab", "test2")

	values, err := cache.ListByPrefix("")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}
	expected := []string{"test1", "test2"}
	compareSlice(t, expected, values)
}

func TestKVCacheSetTwoDepthReverseOrder(t *testing.T) {
	cache := NewKVCache[string, string]()
	// When the order of Set is reversed, first Set will add 2-symbol node,
	// and second set will split it into two.
	cache.Set("ab", "test2")
	cache.Set("a", "test1")

	values, err := cache.ListByPrefix("")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}
	expected := []string{"test1", "test2"}
	compareSlice(t, expected, values)
}

func TestKVCacheSetAppendTail(t *testing.T) {
	cache := NewKVCache[string, string]()
	cache.Set("ab", "test2")
	cache.Set("abc", "test1")

	values, err := cache.ListByPrefix("")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}
	expected := []string{"test2", "test1"}
	compareSlice(t, expected, values)
}

func TestKVCacheSet3(t *testing.T) {
	// Some tests like this here are white-box ones to cover specific code paths,
	// or to check for regressions of fixed issues found by fuzzing.
	cache := NewKVCache[string, string]()
	cache.Set("ab", "test2")
	cache.Set("abc", "test1")
	cache.Set("a", "test4")

	values, err := cache.ListByPrefix("")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}

	expected := []string{
		"test4", "test2", "test1",
	}

	compareSlice(t, expected, values)
}

func TestKVCacheSet4(t *testing.T) {
	cache := NewKVCache[string, string]()
	cache.Set("ab", "test2")
	cache.Set("abc", "test1")
	cache.Set("abz", "test4")

	values, err := cache.ListByPrefix("")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}

	expected := []string{
		"test2", "test1", "test4",
	}

	t.Log(values)
	compareSlice(t, expected, values)
}

func TestKVCacheSet5(t *testing.T) {
	cache := NewKVCache[string, string]()
	cache.Set("abra", "test2")
	cache.Set("cadabra", "test1")
	cache.Set("abracadabra", "test4")

	values, err := cache.ListByPrefix("cad")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}

	expected := []string{
		"test1",
	}

	t.Log(values)
	compareSlice(t, expected, values)
}

func TestKVCacheSetIfPresent(t *testing.T) {
	cache := NewKVCache[string, string]()
	cache.Set("a", "test2")
	cache.Set("b", "test1")
	cache.Set("c", "test4")

	old, inserted := cache.SetIfPresent("a", "test5")
	if !inserted {
		t.Errorf("key \"a\" is present in cache, SetIfPresent should return true")
	}

	if old != "test2" {
		t.Errorf("expected %q, got %q", "test2", old)
	}

	old, inserted = cache.SetIfPresent("a", "test6")
	if !inserted {
		t.Errorf("key \"a\" is present in cache, SetIfPresent should return true")
	}

	if old != "test5" {
		t.Errorf("expected %q, got %q", "test5", old)
	}

	if _, inserted := cache.SetIfPresent("d", "test3"); inserted {
		t.Errorf("key \"d\" is not present in cache, SetIfPresent should return false")
	}

	if _, inserted := cache.SetIfPresent("d", "test3"); inserted {
		t.Errorf("key \"d\" is still not present in cache, SetIfPresent should return false")
	}

	val, err := cache.Get("a")
	if err != nil {
		t.Fatalf("unexpected error in Get: %v", err)
	}

	if val != "test6" {
		t.Errorf("expected %q, got %q", "test6", val)
	}

	_, err = cache.Get("d")
	if err == nil {
		t.Errorf("expected key \"d\" to not be present in the cache")
	}
}

func TestKVCacheSetIfPresentConcurrent(t *testing.T) {
	cache := NewKVCache[string, string]()
	cache.Set("a", "startA")
	cache.Set("b", "startB")

	for i := 0; i < 1000; i++ {
		go func() {
			switch rand.Intn(6) {
			case 0:
				cache.SetIfPresent("a", "a")
			case 1:
				cache.SetIfPresent("b", "b")
			case 2:
				cache.SetIfPresent("c", "c")
			case 3:
				_, _ = cache.Get("a")
			case 4:
				_, _ = cache.Get("b")
			case 5:
				_, _ = cache.Get("c")
			}
		}()
	}

	time.Sleep(10 * time.Millisecond)

	if val, _ := cache.Get("a"); val != "a" {
		t.Errorf("expected %q, got %q", "a", val)
	}

	if val, _ := cache.Get("b"); val != "b" {
		t.Errorf("expected %q, got %q", "b", val)
	}

	if _, err := cache.Get("c"); err == nil {
		t.Errorf("expected key \"c\" to not be present in the cache")
	}
}

func FuzzKVCacheSetListByPrefix(f *testing.F) {
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
		cache := NewKVCache[string, string]()
		for _, key := range golden {
			cache.Set(key, key)
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

		got, err := cache.ListByPrefix(prefix)
		if err != nil {
			t.Fatalf("unexpected error in ListByPrefix: %v", err)
		}

		t.Logf("params: (%q, %q, %q, %q)", k1, k2, k3, prefix)
		t.Logf("got: %v", got)

		compareSlice(t, expect, got)
	})
}

func TestKVCacheDelNoprefix(t *testing.T) {
	cache := NewKVCache[string, string]()
	cache.Set("hu", "hu")
	_ = cache.Del("h")
	res, err := cache.Get("hu")
	if err != nil {
		t.Errorf("unexpected error in Get: %v", err)
	}
	if res != "hu" {
		t.Errorf("expected %q, got %q", "hu", res)
	}
	l, err := cache.ListByPrefix("")
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

func FuzzKVCacheMonkey(f *testing.F) {
	// More elaborate fuzzing test. It creates a random task of 50 Set/Del
	// commands to be executed on a KVCache. Then it checks that ListByPrefix
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
		cache := NewKVCache[string, string]()
		task := randTask(seed)
		golden := make(map[string]struct{}, len(task))
		for _, cmd := range task {
			switch cmd.action {
			case "Set":
				cache.Set(cmd.key, cmd.key)
				golden[cmd.key] = struct{}{}
			case "Del":
				// Since keys are random we expect a lot of Del to fail.
				_ = cache.Del(cmd.key)
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

		got, err := cache.ListByPrefix(prefix)
		if err != nil {
			t.Fatalf("unexpected error in ListByPrefix: %v", err)
		}

		if cache.Len() != len(golden) {
			t.Errorf("expected len %d, got %d", len(golden), cache.Len())
		}

		for _, key := range goldenFiltered {
			val, err := cache.Get(key)
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

func TestKVCacheLen(t *testing.T) {
	cache := NewKVCache[string, string]()

	if cache.Len() != 0 {
		t.Errorf("expected len 0, got %d", cache.Len())
	}

	cache.Set("a", "1")
	if cache.Len() != 1 {
		t.Errorf("expected len 1, got %d", cache.Len())
	}

	cache.Set("b", "2")
	cache.Set("c", "3")
	if cache.Len() != 3 {
		t.Errorf("expected len 3, got %d", cache.Len())
	}

	cache.Set("a", "10")
	if cache.Len() != 3 {
		t.Errorf("expected len 3 after update, got %d", cache.Len())
	}

	_ = cache.Del("b")
	if cache.Len() != 2 {
		t.Errorf("expected len 2 after delete, got %d", cache.Len())
	}

	_ = cache.Del("nonexistent")
	if cache.Len() != 2 {
		t.Errorf("expected len 2 after deleting nonexistent key, got %d", cache.Len())
	}
}

func TestKVCacheFreelist(t *testing.T) {
	cache := NewKVCache[string, string]()

	// Add some values
	cache.Set("a", "1")
	cache.Set("b", "2")
	cache.Set("c", "3")

	if len(cache.freelist) != 0 {
		t.Errorf("expected freelist len 0, got %d", len(cache.freelist))
	}

	// Delete a value - should add to freelist
	_ = cache.Del("b")

	if len(cache.freelist) != 1 {
		t.Errorf("expected freelist len 1 after delete, got %d", len(cache.freelist))
	}

	// Add a new value - should reuse from freelist
	cache.Set("d", "4")

	if len(cache.freelist) != 0 {
		t.Errorf("expected freelist len 0 after reuse, got %d", len(cache.freelist))
	}

	// Verify values
	val, err := cache.Get("d")
	if err != nil {
		t.Fatalf("unexpected error in Get: %v", err)
	}
	if val != "4" {
		t.Errorf("expected %q, got %q", "4", val)
	}

	// Old deleted value should not be accessible
	if _, err := cache.Get("b"); err == nil {
		t.Errorf("expected error for deleted key, got nil")
	}
}

func TestKVCacheConcurrent(t *testing.T) {
	cache := NewKVCache[string, int]()

	// Concurrent writes
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j)
				cache.Set(key, id*100+j)
			}
			done <- true
		}(i)
	}

	// Wait for all writes to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j)
				val, err := cache.Get(key)
				if err != nil {
					t.Errorf("unexpected error in Get: %v", err)
				}
				expected := id*100 + j
				if val != expected {
					t.Errorf("expected %d, got %d", expected, val)
				}
			}
			done <- true
		}(i)
	}

	// Wait for all reads to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestKVCacheWithDifferentTypes(t *testing.T) {
	// Test with int values
	intCache := NewKVCache[string, int]()
	intCache.Set("one", 1)
	intCache.Set("two", 2)

	val, err := intCache.Get("one")
	if err != nil {
		t.Fatalf("unexpected error in Get: %v", err)
	}
	if val != 1 {
		t.Errorf("expected 1, got %d", val)
	}

	// Test with struct values
	type testStruct struct {
		Name string
		Age  int
	}

	structCache := NewKVCache[string, testStruct]()
	structCache.Set("person1", testStruct{Name: "Alice", Age: 30})
	structCache.Set("person2", testStruct{Name: "Bob", Age: 25})

	person, err := structCache.Get("person1")
	if err != nil {
		t.Fatalf("unexpected error in Get: %v", err)
	}
	if person.Name != "Alice" || person.Age != 30 {
		t.Errorf("expected Alice/30, got %s/%d", person.Name, person.Age)
	}

	people, err := structCache.ListByPrefix("person")
	if err != nil {
		t.Fatalf("unexpected error in ListByPrefix: %v", err)
	}
	if len(people) != 2 {
		t.Errorf("expected 2 people, got %d", len(people))
	}
}

func ExampleKVCache_AllByPrefix() {
	cache := NewKVCache[string, string]()

	cache.Set("foo", "bar")
	cache.Set("foo2", "bar2")
	cache.Set("foo3", "bar3")
	cache.Set("foo1", "bar1")

	for k,v := range cache.AllByPrefix("foo") {
	fmt.Println(k, v)
	}
	// Output:
	// foo bar
	// foo1 bar1
	// foo2 bar2
	// foo3 bar3
}

func TestKVCache_AllByPrefix(t *testing.T) {
	cache := NewKVCache[string, string]()

	for i := 999; i >= 0; i-- {
		key := fmt.Sprintf("%03d", i)
		if key == "008" {
			cache.Set(key, key)
		}
		cache.Set(key, key)
	}

	expected := []string{
		"000", "001", "002", "003", "004", "005", "006", "007", "008", "009",
	}

	var got []string
	var gotKeys []string
	seq := cache.AllByPrefix("00")
	seq(func(k, v string) bool {
		gotKeys = append(gotKeys, k)
		got = append(got, v)
		return true
	})
	compareSlice(t, expected, got)
	compareSlice(t, expected, gotKeys)

	expected = []string{
		"120", "121", "122", "123", "124", "125", "126", "127", "128", "129",
	}

	got = nil
	gotKeys = nil
	seq = cache.AllByPrefix("12")
	seq(func(k, v string) bool {
		gotKeys = append(gotKeys, k)
		got = append(got, v)
		return true
	})
	compareSlice(t, expected, got)
	compareSlice(t, expected, gotKeys)

	expected = []string{"888"}

	got = nil
	gotKeys = nil
	seq = cache.AllByPrefix("888")
	seq(func(k, v string) bool {
		gotKeys = append(gotKeys, k)
		got = append(got, v)
		return true
	})
	compareSlice(t, expected, got)
	compareSlice(t, expected, gotKeys)

	_ = cache.Del("777")
	_ = cache.Del("779")

	if _, err := cache.Get("777"); err != ErrNotFound {
		t.Fatalf("expected error %v, got %v", ErrNotFound, err)
	}

	expected = []string{
		"770", "771", "772", "773", "774", "775", "776", "778",
	}
	expectedKeys := []string{
		"770", "771", "772", "773", "774", "775", "776", "778",
	}

	got = nil
	gotKeys = nil
	seq = cache.AllByPrefix("77")
	seq(func(k, v string) bool {
		gotKeys = append(gotKeys, k)
		got = append(got, v)
		return true
	})

	cache.Set("777", "777")
	cache.Set("779", "779")

	compareSlice(t, expected, got)
	compareSlice(t, expectedKeys, gotKeys)

	expected = []string{
		"770", "771", "772", "773", "774", "775", "776", "777", "778", "779",
	}

	got = nil
	gotKeys = nil
	seq = cache.AllByPrefix("77")
	seq(func(k, v string) bool {
		gotKeys = append(gotKeys, k)
		got = append(got, v)
		return true
	})

	compareSlice(t, expected, got)
	compareSlice(t, expected, gotKeys)

	cache.Set("77", "77")

	expected = []string{
		"77", "770", "771", "772", "773", "774", "775", "776", "777", "778", "779",
	}

	got = nil
	gotKeys = nil
	seq = cache.AllByPrefix("77")
	seq(func(k, v string) bool {
		gotKeys = append(gotKeys, k)
		got = append(got, v)
		return true
	})

	compareSlice(t, expected, got)
	compareSlice(t, expected, gotKeys)
}

func TestKVCacheEmptyPrefix_AllByPrefix(t *testing.T) {
	cache := NewKVCache[string, string]()

	expected := []string{}
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("%02d", i)
		expected = append(expected, key)
		cache.Set(key, key)
	}

	var got []string
	var gotKeys []string
	seq := cache.AllByPrefix("")
	seq(func(k, v string) bool {
		gotKeys = append(gotKeys, k)
		got = append(got, v)
		return true
	})

	compareSlice(t, expected, got)
	compareSlice(t, expected, gotKeys)
}

func TestKVCacheEmptyPrefixDiffLen_AllByPrefix(t *testing.T) {
	cache := NewKVCache[string, string]()

	cache.Set("12345", "12345")
	cache.Set("123", "123")
	cache.Set("3", "3")
	cache.Set("2", "2")
	cache.Set("33333", "33333")
	cache.Set("1", "1")

	expected := []string{"1", "123", "12345", "2", "3", "33333"}

	var got []string
	var gotKeys []string
	seq := cache.AllByPrefix("")
	seq(func(k, v string) bool {
		gotKeys = append(gotKeys, k)
		got = append(got, v)
		return true
	})

	compareSlice(t, expected, got)
	compareSlice(t, expected, gotKeys)
}

func TestKVCacheEmptyPrefixFuzz_AllByPrefix(t *testing.T) {
	cache := NewKVCache[string, string]()

	set := map[string]struct{}{}
	for i := 0; i < 10000; i++ {
		key := genRandomString(rand.Intn(300) + 1)
		set[key] = struct{}{}
		cache.Set(key, key)
	}

	expected := []string{}
	for key := range set {
		expected = append(expected, key)
	}
	sort.Strings(expected)

	var got []string
	var gotKeys []string
	seq := cache.AllByPrefix("")
	seq(func(k, v string) bool {
		gotKeys = append(gotKeys, k)
		got = append(got, v)
		return true
	})

	compareSlice(t, expected, got)
	compareSlice(t, expected, gotKeys)
}

func TestKVCachePrefixFuzz_AllByPrefix(t *testing.T) {
	prefixes := []string{}
	for i := 0; i < 10; i++ {
		prefixes = append(prefixes, genRandomString(rand.Intn(20)+1))
	}
	cache := NewKVCache[string, string]()

	set := map[string]struct{}{}
	for i := 0; i < 10000; i++ {
		prefix := prefixes[rand.Intn(len(prefixes))]
		pl := rand.Intn(len(prefix))
		key := prefix[:pl] + genRandomString(rand.Intn(300)+1)
		set[key] = struct{}{}
		cache.Set(key, key)
	}

	// Delete 10% of keys.
	for key := range set {
		if rand.Float64() < 0.1 {
			delete(set, key)
			_ = cache.Del(key)
		}
	}

	expected := []string{}
	for key := range set {
		expected = append(expected, key)
	}
	sort.Strings(expected)

	var got []string
	var gotKeys []string
	seq := cache.AllByPrefix("")
	seq(func(k, v string) bool {
		gotKeys = append(gotKeys, k)
		got = append(got, v)
		return true
	})

	compareSlice(t, expected, got)
	compareSlice(t, expected, gotKeys)

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

			got = nil
			gotKeys = nil
			seq = cache.AllByPrefix(q)
			seq(func(k, v string) bool {
				gotKeys = append(gotKeys, k)
				got = append(got, v)
				return true
			})

			compareSlice(t, expected2, got)
			compareSlice(t, expected2, gotKeys)
		}
	}
}

func TestKVCacheNonexist_AllByPrefix(t *testing.T) {
	cache := NewKVCache[string, string]()

	cache.Set("test", "best")

	var got []string
	seq := cache.AllByPrefix("nonexistent")
	seq(func(k, v string) bool {
		got = append(got, v)
		return true
	})

	if len(got) > 0 {
		t.Errorf("unexpected len %d", len(got))
	}

	if err := cache.Del("nonexistent"); err != nil {
		t.Errorf("unexpected error in Del: %v", err)
	}
}

func TestKVCacheEmptyKey_AllByPrefix(t *testing.T) {
	cache := NewKVCache[string, string]()

	cache.Set("", "0")
	cache.Set("foo1", "1")
	cache.Set("foo2", "2")

	var got []string
	seq := cache.AllByPrefix("fo")
	seq(func(k, v string) bool {
		got = append(got, v)
		return true
	})

	if len(got) > 3 {
		t.Errorf("unexpected len %d", len(got))
	}
}

func TestKVCacheAlloc_AllByPrefix(t *testing.T) {
	cache := NewKVCache[string, string]()

	var (
		mBefore, mAfter runtime.MemStats
		rawDataLen      int64
	)
	runtime.GC()
	runtime.ReadMemStats(&mBefore)

	for i := 0; i < 1_000_000; i++ {
		key := genRandomString(rand.Intn(300) + 1)
		rawDataLen += int64(len(key) * 2)

		cache.Set(key, key)
	}

	runtime.GC()
	runtime.ReadMemStats(&mAfter)
	t.Logf("rawDataLen: %d", rawDataLen)
	t.Logf("memIncrease: %d", mAfter.HeapAlloc-mBefore.HeapAlloc)
	t.Logf("memIncreaseRatio: %0.1f", float64(mAfter.HeapAlloc-mBefore.HeapAlloc)/float64(rawDataLen))

	var keys []string
	seq := cache.AllByPrefix("")
	seq(func(k, v string) bool {
		keys = append(keys, k)
		return true
	})

	for _, key := range keys {
		_ = cache.Del(key)
	}

	runtime.GC()
	runtime.ReadMemStats(&mAfter)
	t.Logf("memIncreaseAfterDel: %d", mAfter.HeapAlloc-mBefore.HeapAlloc)

	if mAfter.HeapAlloc > mBefore.HeapAlloc {
		if mAfter.HeapAlloc-mBefore.HeapAlloc > uint64(rawDataLen) {
			t.Errorf("memory increase is too big")
		}
	}

	if len(cache.trie.children) > 0 {
		t.Log(cache.trie.children)
		t.Errorf("trie is not empty")
	}

	if cache.Len() > 0 {
		t.Errorf("data is not empty")
	}
}

func TestKVCacheDel_AllByPrefix(t *testing.T) {
	cache := NewKVCache[string, string]()

	cache.Set("foo", "bar")
	_ = cache.Del("foo")

	if _, err := cache.Get("foo"); err == nil {
		t.Error("expected error after deleting a key, got nil")
	}

	cache.Set("fo", "bar")
	cache.Set("food", "bar")
	_ = cache.Del("food")

	var res []string
	seq := cache.AllByPrefix("foo")
	seq(func(k, v string) bool {
		res = append(res, v)
		return true
	})
	if len(res) != 0 {
		t.Errorf("expected 0 results, got %d", len(res))
	}

	res = nil
	seq = cache.AllByPrefix("fo")
	seq(func(k, v string) bool {
		res = append(res, v)
		return true
	})
	if len(res) != 1 {
		t.Errorf("expected 1 results, got %d", len(res))
	}
}

func TestKVCacheSetEmptyKey_AllByPrefix(t *testing.T) {
	cache := NewKVCache[string, string]()
	cache.Set("", "test")

	got, err := cache.Get("")
	if err != nil {
		t.Fatalf("unexpected error in Get: %v", err)
	}
	if got != "test" {
		t.Errorf("expected %q, got %q", "test", got)
	}

	var values []string
	var keys []string
	seq := cache.AllByPrefix("")
	seq(func(k, v string) bool {
		keys = append(keys, k)
		values = append(values, v)
		return true
	})
	if len(values) != 1 {
		t.Errorf("expected len %d, got %d", 1, len(values))
	}
	if values[0] != "test" {
		t.Errorf("expected %q, got %q", "test", values[0])
	}
	if keys[0] != "" {
		t.Errorf("expected empty key, got %q", keys[0])
	}
}

func TestKVCacheTwoSingleChar_AllByPrefix(t *testing.T) {
	cache := NewKVCache[string, string]()
	cache.Set("a", "test1")
	cache.Set("b", "test2")

	var values []string
	var keys []string
	seq := cache.AllByPrefix("")
	seq(func(k, v string) bool {
		keys = append(keys, k)
		values = append(values, v)
		return true
	})
	expectedValues := []string{"test1", "test2"}
	expectedKeys := []string{"a", "b"}
	compareSlice(t, expectedValues, values)
	compareSlice(t, expectedKeys, keys)
}

func TestKVCacheSetTwoDepth_AllByPrefix(t *testing.T) {
	cache := NewKVCache[string, string]()
	cache.Set("a", "test1")
	cache.Set("ab", "test2")

	var values []string
	var keys []string
	seq := cache.AllByPrefix("")
	seq(func(k, v string) bool {
		keys = append(keys, k)
		values = append(values, v)
		return true
	})
	expectedValues := []string{"test1", "test2"}
	expectedKeys := []string{"a", "ab"}
	compareSlice(t, expectedValues, values)
	compareSlice(t, expectedKeys, keys)
}

func TestKVCacheSetTwoDepthReverseOrder_AllByPrefix(t *testing.T) {
	cache := NewKVCache[string, string]()
	// When the order of Set is reversed, first Set will add 2-symbol node,
	// and second set will split it into two.
	cache.Set("ab", "test2")
	cache.Set("a", "test1")

	var values []string
	var keys []string
	seq := cache.AllByPrefix("")
	seq(func(k, v string) bool {
		keys = append(keys, k)
		values = append(values, v)
		return true
	})
	expectedValues := []string{"test1", "test2"}
	expectedKeys := []string{"a", "ab"}
	compareSlice(t, expectedValues, values)
	compareSlice(t, expectedKeys, keys)
}

func TestKVCacheSetAppendTail_AllByPrefix(t *testing.T) {
	cache := NewKVCache[string, string]()
	cache.Set("ab", "test2")
	cache.Set("abc", "test1")

	var values []string
	var keys []string
	seq := cache.AllByPrefix("")
	seq(func(k, v string) bool {
		keys = append(keys, k)
		values = append(values, v)
		return true
	})
	expectedValues := []string{"test2", "test1"}
	expectedKeys := []string{"ab", "abc"}
	compareSlice(t, expectedValues, values)
	compareSlice(t, expectedKeys, keys)
}

func TestKVCacheSet3_AllByPrefix(t *testing.T) {
	// Some tests like this here are white-box ones to cover specific code paths,
	// or to check for regressions of fixed issues found by fuzzing.
	cache := NewKVCache[string, string]()
	cache.Set("ab", "test2")
	cache.Set("abc", "test1")
	cache.Set("a", "test4")

	var values []string
	var keys []string
	seq := cache.AllByPrefix("")
	seq(func(k, v string) bool {
		keys = append(keys, k)
		values = append(values, v)
		return true
	})

	expectedValues := []string{
		"test4", "test2", "test1",
	}
	expectedKeys := []string{
		"a", "ab", "abc",
	}

	compareSlice(t, expectedValues, values)
	compareSlice(t, expectedKeys, keys)
}

func TestKVCacheSet4_AllByPrefix(t *testing.T) {
	cache := NewKVCache[string, string]()
	cache.Set("ab", "test2")
	cache.Set("abc", "test1")
	cache.Set("abz", "test4")

	var values []string
	var keys []string
	seq := cache.AllByPrefix("")
	seq(func(k, v string) bool {
		keys = append(keys, k)
		values = append(values, v)
		return true
	})

	expectedValues := []string{
		"test2", "test1", "test4",
	}
	expectedKeys := []string{
		"ab", "abc", "abz",
	}

	t.Log(values)
	compareSlice(t, expectedValues, values)
	compareSlice(t, expectedKeys, keys)
}

func TestKVCacheSet5_AllByPrefix(t *testing.T) {
	cache := NewKVCache[string, string]()
	cache.Set("abra", "test2")
	cache.Set("cadabra", "test1")
	cache.Set("abracadabra", "test4")

	var values []string
	var keys []string
	seq := cache.AllByPrefix("cad")
	seq(func(k, v string) bool {
		keys = append(keys, k)
		values = append(values, v)
		return true
	})

	expectedValues := []string{
		"test1",
	}
	expectedKeys := []string{
		"cadabra",
	}

	t.Log(values)
	compareSlice(t, expectedValues, values)
	compareSlice(t, expectedKeys, keys)
}

func FuzzKVCacheSetAllByPrefix(f *testing.F) {
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
		cache := NewKVCache[string, string]()
		for _, key := range golden {
			cache.Set(key, key)
		}

		sort.Strings(golden)

		expectValues := make([]string, 0, len(golden))
		expectKeys := make([]string, 0, len(golden))

		seen := map[string]struct{}{}
		for _, s := range golden {
			if _, ok := seen[s]; !ok && strings.HasPrefix(s, prefix) {
				expectValues = append(expectValues, s)
				expectKeys = append(expectKeys, s)
				seen[s] = struct{}{}
			}
		}

		var gotValues []string
		var gotKeys []string
		seq := cache.AllByPrefix(prefix)
		seq(func(k, v string) bool {
			gotKeys = append(gotKeys, k)
			gotValues = append(gotValues, v)
			return true
		})

		t.Logf("params: (%q, %q, %q, %q)", k1, k2, k3, prefix)
		t.Logf("got: %v", gotValues)

		compareSlice(t, expectValues, gotValues)
		compareSlice(t, expectKeys, gotKeys)
	})
}

func TestKVCacheDelNoprefix_AllByPrefix(t *testing.T) {
	cache := NewKVCache[string, string]()
	cache.Set("hu", "hu")
	_ = cache.Del("h")
	res, err := cache.Get("hu")
	if err != nil {
		t.Errorf("unexpected error in Get: %v", err)
	}
	if res != "hu" {
		t.Errorf("expected %q, got %q", "hu", res)
	}

	var l []string
	seq := cache.AllByPrefix("")
	seq(func(k, v string) bool {
		l = append(l, v)
		return true
	})

	if len(l) != 1 {
		t.Fatalf("expected len 1, got %d", len(l))
	}

	if l[0] != "hu" {
		t.Errorf("expected %q, got %q", "hu", res)
	}
}

func FuzzKVCacheMonkey_AllByPrefix(f *testing.F) {
	// More elaborate fuzzing test. It creates a random task of 50 Set/Del
	// commands to be executed on a KVCache. Then it checks that AllByPrefix
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
		cache := NewKVCache[string, string]()
		task := randTask(seed)
		golden := make(map[string]struct{}, len(task))
		for _, cmd := range task {
			switch cmd.action {
			case "Set":
				cache.Set(cmd.key, cmd.key)
				golden[cmd.key] = struct{}{}
			case "Del":
				// Since keys are random we expect a lot of Del to fail.
				_ = cache.Del(cmd.key)
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

		var gotValues []string
		var gotKeys []string
		seq := cache.AllByPrefix(prefix)
		seq(func(k, v string) bool {
			gotKeys = append(gotKeys, k)
			gotValues = append(gotValues, v)
			return true
		})

		if cache.Len() != len(golden) {
			t.Errorf("expected len %d, got %d", len(golden), cache.Len())
		}

		for _, key := range goldenFiltered {
			val, err := cache.Get(key)
			if err != nil {
				t.Fatalf("unexpected error in Get: %v", err)
			}
			if val != key {
				t.Errorf("expected %q, got %q", key, val)
			}
		}

		t.Logf("seed: %d, task %v, prefix: %q", seed, task, prefix)
		compareSlice(t, goldenFiltered, gotValues)
		compareSlice(t, goldenFiltered, gotKeys)
	})
}

func TestKVCacheWithDifferentTypes_AllByPrefix(t *testing.T) {
	// Test with int values
	intCache := NewKVCache[string, int]()
	intCache.Set("one", 1)
	intCache.Set("two", 2)

	val, err := intCache.Get("one")
	if err != nil {
		t.Fatalf("unexpected error in Get: %v", err)
	}
	if val != 1 {
		t.Errorf("expected 1, got %d", val)
	}

	// Test with struct values
	type testStruct struct {
		Name string
		Age  int
	}

	structCache := NewKVCache[string, testStruct]()
	structCache.Set("person1", testStruct{Name: "Alice", Age: 30})
	structCache.Set("person2", testStruct{Name: "Bob", Age: 25})

	person, err := structCache.Get("person1")
	if err != nil {
		t.Fatalf("unexpected error in Get: %v", err)
	}
	if person.Name != "Alice" || person.Age != 30 {
		t.Errorf("expected Alice/30, got %s/%d", person.Name, person.Age)
	}

	var people []testStruct
	var keys []string
	seq := structCache.AllByPrefix("person")
	seq(func(k string, v testStruct) bool {
		keys = append(keys, k)
		people = append(people, v)
		return true
	})

	if len(people) != 2 {
		t.Errorf("expected 2 people, got %d", len(people))
	}
	expectedKeys := []string{"person1", "person2"}
	compareSlice(t, expectedKeys, keys)
}

func TestKVCacheSnapshot(t *testing.T) {
	cache := NewKVCache[string, string]()

	// 1. Add some data
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("prefix/key3", "value3")

	// 2. Take a snapshot
	snapshotMap := cache.Snapshot()

	// 3. Verify content of the snapshot
	expected := map[string]string{
		"key1":        "value1",
		"key2":        "value2",
		"prefix/key3": "value3",
	}

	if len(snapshotMap) != len(expected) {
		t.Errorf("Snapshot map has unexpected length. Expected %d, got %d", len(expected), len(snapshotMap))
	}

	for k, v := range expected {
		if gotV, ok := snapshotMap[k]; !ok || gotV != v {
			t.Errorf("Snapshot missing key %q or has wrong value. Expected %q, got %q", k, v, gotV)
		}
	}

	// 4. Verify independence: modify original cache, snapshot should be unchanged
	cache.Set("key4", "value4") // Add a new item
	_ = cache.Del("key1")       // Delete an existing item
	cache.Set("key2", "newValue2") // Update an existing item

	// Snapshot should still reflect the state at the time it was taken
	if len(snapshotMap) != len(expected) { // Length should still be 3
		t.Errorf("Snapshot map length changed after original cache modification. Expected %d, got %d", len(expected), len(snapshotMap))
	}
	if _, ok := snapshotMap["key4"]; ok { // new key4 should not be in snapshot
		t.Errorf("Snapshot contains new key 'key4' which was added after snapshot")
	}
	if gotV, ok := snapshotMap["key1"]; !ok || gotV != "value1" { // key1 should still be in snapshot with old value
		t.Errorf("Snapshot's 'key1' was affected by deletion in original. Expected %q, got %q", "value1", gotV)
	}
	if gotV, ok := snapshotMap["key2"]; !ok || gotV != "value2" { // key2 should still have old value
		t.Errorf("Snapshot's 'key2' was affected by update in original. Expected %q, got %q", "value2", gotV)
	}

	// Also test an empty cache snapshot
	emptyCache := NewKVCache[string, string]()
	emptySnapshot := emptyCache.Snapshot()
	if len(emptySnapshot) != 0 {
		t.Errorf("Empty cache snapshot should be empty, got length %d", len(emptySnapshot))
	}
}

func TestKVCache_DeleteMerge(t *testing.T) {
	kv := NewKVCache[string, string]()
	kv.Set("apple", "fruit")
	kv.Set("apply", "verb")

	// Initial state:
	// root -> "appl" -> "e" (val=fruit)
	//                -> "y" (val=verb)

	// Delete "apple"
	_ = kv.Del("apple")

	// Expected state if merged:
	// root -> "apply" (val=verb)

	// Verify "apply" is still accessible
	val, err := kv.Get("apply")
	if err != nil {
		t.Fatalf("Get('apply') failed: %v", err)
	}
	if val != "verb" {
		t.Errorf("Expected 'verb', got '%s'", val)
	}

	// Verify "apple" is gone
	_, err = kv.Get("apple")
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound for 'apple', got %v", err)
	}

	// Verify iteration works
	items := kv.Snapshot()
	if len(items) != 1 {
		t.Errorf("Expected 1 item, got %d", len(items))
	}
	if items["apply"] != "verb" {
		t.Errorf("Snapshot content mismatch")
	}
}

func TestKVCache_DeleteEmpty(t *testing.T) {
	kv := NewKVCache[string, string]()
	kv.Set("a", "val_a")
	kv.Set("b", "val_b")

	// Delete "a" - this should leave an empty node for "a" which needs cleanup
	_ = kv.Del("a")

	val, err := kv.Get("b")
	if err != nil {
		t.Fatalf("Get('b') failed: %v", err)
	}
	if val != "val_b" {
		t.Errorf("Expected 'val_b', got '%s'", val)
	}

	_, err = kv.Get("a")
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound for 'a', got %v", err)
	}
}
