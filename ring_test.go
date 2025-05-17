package geche

import (
	"math/rand"
	"strconv"
	"testing"
)

func TestRing(t *testing.T) {
	c := NewRingBuffer[string, string](10)

	for i := 0; i < 15; i++ {
		s := strconv.Itoa(i)
		c.Set(s, s)
	}

	// Check the value does not exist (overwritten).
	for i := 0; i < 5; i++ {
		s := strconv.Itoa(i)
		if _, err := c.Get(s); err != ErrNotFound {
			t.Errorf("Get(%q): expected error %v, but got %v", s, ErrNotFound, err)
		}
	}

	expected := []struct {
		K string
		V string
	}{
		{"5", "5"},
		{"6", "6"},
		{"7", "7"},
		{"8", "8"},
		{"9", "9"},
		{"10", "10"},
		{"11", "11"},
		{"12", "12"},
		{"13", "13"},
		{"14", "14"},
	}

	got := c.ListAll()
	if len(got) != len(expected) {
		t.Errorf("expected %d items, but got %d", len(expected), len(got))
	}

	for i, item := range got {
		if item.K != expected[i].K || item.V != expected[i].V {
			t.Errorf(
				"expected item %s:%s, but got %s:%s",
				expected[i].K, expected[i].V,
				item.K, item.V,
			)
		}
	}

	// Check we can get the value.
	checkExistingKeys := func() {
		for i := 5; i < 15; i++ {
			s := strconv.Itoa(i)
			val, err := c.Get(s)
			if err != nil {
				t.Errorf("unexpected error in Get(%q): %v", s, err)
			}

			if val != s {
				t.Errorf("expected value %q, but got %q", s, val)
			}
		}
	}
	checkExistingKeys()

	// SetIfPresent on existing key or non-existing key does not result in eviction
	_, inserted := c.SetIfPresent(strconv.Itoa(0), strconv.Itoa(0))
	if inserted {
		t.Error("SetIfPresent returned inserted=true for non-existing key 0")
	}

	if _, err := c.Get(strconv.Itoa(0)); err != ErrNotFound {
		t.Errorf("Get(%d): expected ErrNotFound, but got %v", 0, err)
	}

	for _, i := range []int{5, 6, 14} {
		old, inserted := c.SetIfPresent(strconv.Itoa(i), strconv.Itoa(i))
		if !inserted {
			t.Error("SetIfPresent returned inserted=false for existing key")
		}

		if old != strconv.Itoa(i) {
			t.Errorf("SetIfPresent returned incorrect old value, expected %d, got %s", i, old)
		}
	}

	checkExistingKeys()
}

func TestRingListAll(t *testing.T) {
	c := NewRingBuffer[int, int](10)

	// This will emulate the expected behavior of the ring buffer.
	slice := make([]int, 0, 10)

	for i := 0; i < 10000; i++ {
		c.Set(i, i)

		slice = append(slice, i)
		if len(slice) > 10 {
			slice = slice[1:]
		}

		if rand.Intn(10) == 0 {
			// Randomly remove an item from the ring buffer.
			toDelIdx := rand.Intn(len(slice))
			toDelKey := slice[toDelIdx]
			c.Del(toDelKey)
			// Mark item as deleted in the slice.
			// Acrually deleting from the slice wont'b reflect in the ring buffer,
			// as deleted value still takes space in the ring buffer.
			slice[toDelIdx] = -1
		}

		expected := make([]BufferRec[int, int], 0, len(slice))
		for _, v := range slice {
			if v == -1 {
				continue
			}
			expected = append(expected, BufferRec[int, int]{K: v, V: v})
		}

		got := c.ListAll()
		if len(got) != len(expected) {
			t.Fatalf("expected %d items, but got %d", len(expected), len(got))
		}

		for j, item := range got {
			if item.K != expected[j].K || item.V != expected[j].V {
				t.Fatalf(
					"expected item %d:%d, but got %d:%d",
					expected[j].K, expected[j].V,
					item.K, item.V,
				)
			}
		}
	}
}

func TestRingListAllValues(t *testing.T) {
	c := NewRingBuffer[int, int](10)

	// This will emulate the expected behavior of the ring buffer.
	slice := make([]int, 0, 10)

	for i := 0; i < 10000; i++ {
		c.Set(i, i)

		slice = append(slice, i)
		if len(slice) > 10 {
			slice = slice[1:]
		}

		if rand.Intn(10) == 0 {
			// Randomly remove an item from the ring buffer.
			toDelIdx := rand.Intn(len(slice))
			toDelKey := slice[toDelIdx]
			c.Del(toDelKey)
			// Mark item as deleted in the slice.
			slice[toDelIdx] = -1
		}

		expectedValues := make([]int, 0, len(slice))
		for _, v := range slice {
			if v == -1 {
				continue
			}
			expectedValues = append(expectedValues, v)
		}

		gotValues := c.ListAllValues()
		if len(gotValues) != len(expectedValues) {
			t.Fatalf("expected %d values, but got %d", len(expectedValues), len(gotValues))
		}

		for j, val := range gotValues {
			if val != expectedValues[j] {
				t.Fatalf(
					"expected value %d, but got %d",
					expectedValues[j], val,
				)
			}
		}
	}
}

func TestRingListAllKeys(t *testing.T) {
	c := NewRingBuffer[int, int](10)

	// This will emulate the expected behavior of the ring buffer.
	slice := make([]int, 0, 10)

	for i := 0; i < 10000; i++ {
		c.Set(i, i)

		slice = append(slice, i)
		if len(slice) > 10 {
			slice = slice[1:]
		}

		if rand.Intn(10) == 0 {
			// Randomly remove an item from the ring buffer.
			toDelIdx := rand.Intn(len(slice))
			toDelKey := slice[toDelIdx]
			c.Del(toDelKey)
			// Mark item as deleted in the slice.
			slice[toDelIdx] = -1
		}

		expectedKeys := make([]int, 0, len(slice))
		for _, v := range slice {
			if v == -1 {
				continue
			}
			expectedKeys = append(expectedKeys, v)
		}

		gotKeys := c.ListAllKeys()
		if len(gotKeys) != len(expectedKeys) {
			t.Fatalf("expected %d keys, but got %d", len(expectedKeys), len(gotKeys))
		}

		for j, key := range gotKeys {
			if key != expectedKeys[j] {
				t.Fatalf(
					"expected key %d, but got %d",
					expectedKeys[j], key,
				)
			}
		}
	}
}

func TestRingAllIterator(t *testing.T) {
	c := NewRingBuffer[int, int](10)

	// This will emulate the expected behavior of the ring buffer.
	slice := make([]int, 0, 10)

	for i := 0; i < 10000; i++ {
		c.Set(i, i)

		slice = append(slice, i)
		if len(slice) > 10 {
			slice = slice[1:]
		}

		if rand.Intn(10) == 0 {
			// Randomly remove an item from the ring buffer.
			toDelIdx := rand.Intn(len(slice))
			toDelKey := slice[toDelIdx]
			c.Del(toDelKey)
			// Mark item as deleted in the slice.
			slice[toDelIdx] = -1
		}

		expectedItems := make([]BufferRec[int, int], 0, len(slice))
		for _, v := range slice {
			if v == -1 {
				continue
			}
			expectedItems = append(expectedItems, BufferRec[int, int]{K: v, V: v})
		}

		var gotItems []BufferRec[int, int]
		for k, v := range c.All() {
			gotItems = append(gotItems, BufferRec[int, int]{K: k, V: v})
		}

		if len(gotItems) != len(expectedItems) {
			t.Fatalf("expected %d items from iterator, but got %d", len(expectedItems), len(gotItems))
		}

		for j, item := range gotItems {
			if item.K != expectedItems[j].K || item.V != expectedItems[j].V {
				t.Fatalf(
					"iterator: expected item %d:%d, but got %d:%d",
					expectedItems[j].K, expectedItems[j].V,
					item.K, item.V,
				)
			}
		}
	}
}

func TestRingListAll_Empty(t *testing.T) {
	c := NewRingBuffer[int, int](5)

	if testing.Verbose() {
		t.Log("Testing ListAll on an empty buffer")
	}

	got := c.ListAll()
	if len(got) != 0 {
		t.Errorf("expected 0 items, but got %d", len(got))
	}
}

func TestRingListAllValues_Empty(t *testing.T) {
	c := NewRingBuffer[int, int](5)
	if testing.Verbose() {
		t.Log("Testing ListAllValues on an empty buffer")
	}
	got := c.ListAllValues()
	if len(got) != 0 {
		t.Errorf("expected 0 values, but got %d", len(got))
	}
}

func TestRingListAllKeys_Empty(t *testing.T) {
	c := NewRingBuffer[int, int](5)
	if testing.Verbose() {
		t.Log("Testing ListAllKeys on an empty buffer")
	}
	got := c.ListAllKeys()
	if len(got) != 0 {
		t.Errorf("expected 0 keys, but got %d", len(got))
	}
}

func TestRingAllIterator_Empty(t *testing.T) {
	c := NewRingBuffer[int, int](5)
	if testing.Verbose() {
		t.Log("Testing All iterator on an empty buffer")
	}
	iter := c.All()
	count := 0
	for range iter {
		count++
	}
	if count != 0 {
		t.Errorf("expected 0 items from iterator, but got %d", count)
	}
}

func TestRingListAll_OneElement(t *testing.T) {
	c := NewRingBuffer[int, int](5)
	c.Set(1, 10)
	if testing.Verbose() {
		t.Log("Testing ListAll with one element")
	}

	expected := []BufferRec[int, int]{{K: 1, V: 10}}
	got := c.ListAll()
	if len(got) != len(expected) {
		t.Errorf("expected %d items, but got %d", len(expected), len(got))
	}
	for i, item := range got {
		if item.K != expected[i].K || item.V != expected[i].V {
			t.Errorf("expected item %d:%d, but got %d:%d", expected[i].K, expected[i].V, item.K, item.V)
		}
	}
}

func TestRingListAllValues_OneElement(t *testing.T) {
	c := NewRingBuffer[int, int](5)
	c.Set(1, 10)
	if testing.Verbose() {
		t.Log("Testing ListAllValues with one element")
	}
	expected := []int{10}
	got := c.ListAllValues()
	if len(got) != len(expected) {
		t.Errorf("expected %d values, but got %d", len(expected), len(got))
	}
	for i, val := range got {
		if val != expected[i] {
			t.Errorf("expected value %d, but got %d", expected[i], val)
		}
	}
}

func TestRingListAllKeys_OneElement(t *testing.T) {
	c := NewRingBuffer[int, int](5)
	c.Set(1, 10)
	if testing.Verbose() {
		t.Log("Testing ListAllKeys with one element")
	}
	expected := []int{1}
	got := c.ListAllKeys()
	if len(got) != len(expected) {
		t.Errorf("expected %d keys, but got %d", len(expected), len(got))
	}
	for i, key := range got {
		if key != expected[i] {
			t.Errorf("expected key %d, but got %d", expected[i], key)
		}
	}
}

func TestRingAllIterator_OneElement(t *testing.T) {
	c := NewRingBuffer[int, int](5)
	c.Set(1, 10)
	if testing.Verbose() {
		t.Log("Testing All iterator with one element")
	}
	expectedItems := []BufferRec[int, int]{{K: 1, V: 10}}
	iter := c.All()
	var gotItems []BufferRec[int, int]
	for k, v := range iter {
		gotItems = append(gotItems, BufferRec[int, int]{K: k, V: v})
	}
	if len(gotItems) != len(expectedItems) {
		t.Errorf("expected %d items from iterator, but got %d", len(expectedItems), len(gotItems))
	}
	for j, item := range gotItems {
		if item.K != expectedItems[j].K || item.V != expectedItems[j].V {
			t.Errorf("iterator: expected item %d:%d, but got %d:%d", expectedItems[j].K, expectedItems[j].V, item.K, item.V)
		}
	}
}

func TestRingListAll_AllDeleted(t *testing.T) {
	c := NewRingBuffer[int, int](5)
	c.Set(1, 10)
	c.Set(2, 20)
	c.Del(1)
	c.Del(2)
	if testing.Verbose() {
		t.Log("Testing ListAll with all elements deleted")
	}
	got := c.ListAll()
	if len(got) != 0 {
		t.Errorf("expected 0 items, but got %d", len(got))
	}
}

func TestRingListAllValues_AllDeleted(t *testing.T) {
	c := NewRingBuffer[int, int](5)
	c.Set(1, 10)
	c.Set(2, 20)
	c.Del(1)
	c.Del(2)
	if testing.Verbose() {
		t.Log("Testing ListAllValues with all elements deleted")
	}
	got := c.ListAllValues()
	if len(got) != 0 {
		t.Errorf("expected 0 values, but got %d", len(got))
	}
}

func TestRingListAllKeys_AllDeleted(t *testing.T) {
	c := NewRingBuffer[int, int](5)
	c.Set(1, 10)
	c.Set(2, 20)
	c.Del(1)
	c.Del(2)
	if testing.Verbose() {
		t.Log("Testing ListAllKeys with all elements deleted")
	}
	got := c.ListAllKeys()
	if len(got) != 0 {
		t.Errorf("expected 0 keys, but got %d", len(got))
	}
}

func TestRingAllIterator_AllDeleted(t *testing.T) {
	c := NewRingBuffer[int, int](5)
	c.Set(1, 10)
	c.Set(2, 20)
	c.Del(1)
	c.Del(2)
	if testing.Verbose() {
		t.Log("Testing All iterator with all elements deleted")
	}
	iter := c.All()
	count := 0
	for range iter {
		count++
	}
	if count != 0 {
		t.Errorf("expected 0 items from iterator, but got %d", count)
	}
}
