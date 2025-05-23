# Geche (generic cache)

[![Workflow status](https://github.com/C-Pro/geche/actions/workflows/test.yml/badge.svg)](https://github.com/C-Pro/geche/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/c-pro/geche)](https://goreportcard.com/report/github.com/c-pro/geche)
[![GoDoc](https://pkg.go.dev/badge/mod/github.com/c-pro/geche)](https://pkg.go.dev/mod/github.com/c-pro/geche)
[![Coverage Status](https://codecov.io/gh/c-pro/geche/branch/main/graph/badge.svg)](https://codecov.io/gh/c-pro/geche)

Collection of generic cache implementations in Go focused on simplicity. No clever tricks, no dependencies, no gazillion of configuration options.
Implementations are as simple as possible to be predictable in max latency, memory allocation and concurrency impact (writes lock reads and are serialized with other writes).

* `MapCache` is a very simple map-based thread-safe cache, that is not limited from growing. Can be used when you have relatively small number of distinct keys that does not grow significantly, and you do not need the values to expire automatically. E.g. if your keys are country codes, timezones etc, this cache type is ok to use.
* `MapTTLCache` is map-based thread-safe cache with support for TTL (values automatically expire). If you don't want to read value from cache that is older than some threshold (e.g. 1 sec), you set this TTL when initializing the cache object and obsolete rows will be removed from cache automatically.
* `RingBuffer` is a predefined size cache that allocates all memory from the start and will not grow above it. It keeps constant size by overwriting the oldest values in the cache with new ones. Use this cache when you need speed and fixed memory footprint, and your key cardinality is predictable (or you are ok with having cache misses if cardinality suddenly grows above your cache size).

## Examples

Interface is quite simple with six methods: `Set`, `Get`, `Del`, `SetIfPresent`, `Snapshot` and `Len`. Here's a quick example for a ring buffer holding 10k records.

```go
package main

import (
    "fmt"

    "github.com/c-pro/geche"
)

func main() {
    // Create ring buffer FIFO cache with size 10k.
    c := geche.NewRingBuffer[int, string](10000)

    c.Set(1, "one")
    c.Set(2, "dua")
    c.Del(2)
    v, err := c.Get(1)
    if err != nil {
        fmt.Println(err)
        return
    }
	
	// will update the value associated to key 1
	previousVal, updated := c.SetIfPresent(1, "two")
	// will print "one"
	fmt.Println(previousVal)
	// will print "true"
	fmt.Println(updated)
	
	// will not have any effect
	c.SetIfPresent(2, "dua")

	// will print 2
    fmt.Println(v)
}
```

Sometimes it is useful to get snapshot of the whole cache (e.g. to avoid cold cache on service restart).
All cache implementations have `Snapshot() map[K]V` function that aquires Read Lock, copies the cache content to the map and returns it. Notice that maps in Go do not guarantee order of keys, so if you need to iterate over the cache in some specific order, see section for `NewKV` wrapper below.

Please be aware that it is a shallow copy, so if your cache value type contains reference types, it may be unsafe to modify the returned copy.

```go
    c := geche.NewMapCache[int, string]()
    c.Set(1, "one")
    c.Set(2, "dua")

    fmt.Println(c.Snapshot())
```

## Wrappers

There are several wrappers that you can use to add some extra features to your cache of choice.
You can nest wrappers, but be aware that order in which you wrap will change the behaviour of resulting cache.

For example if you wrap `NewUpdater` with `NewSharded`, your updater poolSize will be effectively multiplied by number of shards, because each shard will be a separate `Updater` instance.

### CacheUpdater

Another useful wrapper is the `CacheUpdater`. It implements a common scenario, when on cache miss some function is called to get the value from external resource (database, API, some lengthy calculation).
This scenario sounds deceptively simple, but with straightforward implementation can lead to nasty problems like [cache stampede](https://en.wikipedia.org/wiki/Cache_stampede).
To avoid these types of problems `CacheUpdater` has two limiting mechanisms:

* Pool size limits number of keys that can be updated concurrently. E.g. if you set pool size of 10, your cache update will never run more then 10 simultaneous queries to get the value that was not found in the cache.
* In-flight mechanism does not allow to call update function to get the value for the key if the update function for this key is already running. E.g. if suddenly 10 requests for the same key hit the cache and miss (key is not in the cache), only the first one will trigger the update and others will just wait for the result.

```go
updateFn := func(key string) (string, error) {
    return DoSomeDatabaseQuery(key)
}

u := NewCacheUpdater[string, string](
    NewMapCache[string, string](),
    updateFn,
    10,
)

v, err := c.Get("1") // Updater will get value from db here.
if err != nil {
    fmt.Println(err)
    return
}

fmt.Println(v)

v, err = c.Get("1") // And now the value will be returned from the cache.
if err != nil {
    fmt.Println(err)
    return
}

fmt.Println(v)
```

`Updater` provides `ListByPrefix` function, but it can be used only if underlying cache supports it (is a `KV` wrapper).
Otherwize it will panic.

### Sharding

If you intend to use cache in *higlhy* concurrent manner (16+ cores and 100k+ RPS). It may make sense to shard it.
To shard the cache you need to wrap it using `NewSharded`. Sharded cache will determine to which shard the value should go using a mapper that implements interface with `Map(key K, numShards int) int` function. The point of this function is to uniformly map keys to provided number of shards.

```go
func main() {
    // Create sharded TTL cache with number of shards defined automatically
    // based on number of available CPUs.
    c := NewSharded[string](
                    func() Geche[string, string] {
                        return NewMapTTLCache[string, string](ctx, time.Second, time.Second)
                    },
                    0,
                    &StringMapper{},
                )

    c.Set("1", "one")
    c.Set("2", "dua")
    c.Del("2")
    v, err := c.Get("1")
    if err != nil {
        fmt.Println(err)
        return
    }

    fmt.Println(v)
}
```

### KV

If your use-case requires not only random but also sequential access to values in the cache, you can wrap it using `NewKV` wrapper. It will provide you with extra `ListByPrefix` function that returns all values in the cache that have keys starting with provided prefix. Values will be returned in lexicographical order of the keys (order by key).

Another useful trick is to use `ListByPrefix("")` to get all values in the cache as a slice ordered by key.

Internally `KV` maintains trie structure to store keys to be able to quickly find all keys with the same prefix. This trie is updated on every `Set` and `Del` operation, so it is not free both in terms of CPU and memory consumption. If you don't need `ListByPrefix` functionality, don't use this wrapper.

This wrapper has some limitations:
* `KV` only supports keys of type `string`.
* Lexicographical order is maintained on the byte level, so it will work as expected for ASCII strings, but may not work for other encodings.
* `Updater` and `Locker` wrappers provide `ListByPrefix` function, that will call underlying `KV` implementation. But if you wrap `KV` with `Sharded` wrapper, you will loose this functionality. In other words it would not make sense to wrap `KV` with `Sharded` wrapper.

```go
	cache := NewMapCache[string, string]()
	kv := NewKV[string](cache)

	kv.Set("foo", "bar")
	kv.Set("foo2", "bar2")
	kv.Set("foo3", "bar3")
	kv.Set("foo1", "bar1")

	res, _ := kv.ListByPrefix("foo")
	fmt.Println(res)
	// Output: [bar bar1 bar2 bar3]
```

### Locker

This wrapper is useful when you need to make several operations on the cache atomically. For example you store account balances in the cache and want to transfer some amount from one account to another:

```go
	locker := NewLocker[int, int](NewMapCache[int, int]())
    // Acquire RW lock "transaction".
    tx := locker.Lock()

    balA, _ := tx.Get(accA)
    balB, _ := tx.Get(accB)

    amount := 100

    balA += amount
    balB -= amount

    tx.Set(accA, balA)
    tx.Set(accB, balB)

    // Unlock the cache.
    tx.Unlock()
```

The `Locker` itself does not implement `Geche` interface, but `Tx` object returned by `Lock` or `RLock` method does.
Be careful to follow these rules (will lead to panics):
* do not use `Set` and `Del` on read-only `Tx` acquired with `RLock`.
* do not use `Tx` after `Unlock` call.
* do not `Unlock` `Tx` that was unlocked before.
And do not forget to `Unlock` the `Tx` object, otherwise it will lead to lock to be held forever.

Returned `Tx` object is not a transaction in a sense that it does not
allow rollback, but it provides atomicity and isolation guarantees.

`Locker` provides `ListByPrefix` function, but it can only be used if underlying cache implementation supports it (is a `KV` wrapper). Otherwize it will panic.

## Benchmarks

Test suite contains a couple of benchmarks to compare the speed difference between old-school generic implementation using `interface{}` or `any` to hold cache values versus using generics.

TL/DR: generics are faster than `interface{}` but slower than hardcoded type implementation. Ring buffer is 2x+ faster than map-based TTL cache.

There are two types of benchmarks:
* `BenchmarkSet` only times the `Set` operation that allocates all the memory, and usually is the most resource intensive.
* `BenchmarkEverything` repeatedly does one of three operations (Get/Set/Del). The probability for each type of operation to be executed is 0.9/0.05/0.05 respectively. Each operation is executed on randomly generated key, there are totally 1 million distinct keys, so total cache size will be limited too.

Another benchmark `BenchmarkKVListByPrefix` lists `KV` wrapper's `ListByPrefix` operation. It times getting all values matching particular prefix in a cache with 1 million keys. Benchmark is arranged so each call returns 10 records.

Benchmarking four simple cache implementations shows that generic cache (`MapCache`) is faster than cache that uses an empty interface to store any type of values (`AnyCache`), but slower than implementations that use concrete types (`StringCache`) and skip on thread safety (`UnsafeCache`).
Generic `MapTTLCache` is on par with `AnyCache` but it is to be expected as it does more work keeping linked list for fast invalidation. `RingBuffer` performs the best because all the space it needs is preallocated during the initialization, and actual cache size is limited.

Note that `stringCache`, `unsafeCache`, `anyCache` implementations are unexported. These implementations exist only to compare Go generic implementation with other approaches.

The results below are not to be treated as absolute values. Actual cache operation latency will depend on many variables such as CPU speed, key cardinality, number of concurrent operations, whether the allocation happen during the operation or underlying structure already has the allocated space and so on.

```shell
 $ go test -bench=. -benchmem -benchtime=10s .
goos: linux
goarch: amd64
pkg: github.com/c-pro/geche
cpu: Intel(R) Xeon(R) Platinum 8358 CPU @ 2.60GHz
BenchmarkSet/MapCache-32        41473179               284.4 ns/op             1 B/op          0 allocs/op
BenchmarkSet/StringCache-32     64817786               182.5 ns/op             1 B/op          0 allocs/op
BenchmarkSet/UnsafeCache-32     80224212               125.2 ns/op             1 B/op          0 allocs/op
BenchmarkSet/MapTTLCache-32     14296934               758.3 ns/op            15 B/op          0 allocs/op
BenchmarkSet/RingBuffer-32      64152157               244.9 ns/op             0 B/op          0 allocs/op
BenchmarkSet/KVMapCache-32      10701508              1152 ns/op              10 B/op          0 allocs/op
BenchmarkSet/AnyCache-32        67699846               288.9 ns/op             2 B/op          0 allocs/op
BenchmarkEverything/MapCache-32                 100000000              106.7 ns/op             0 B/op          0 allocs/op
BenchmarkEverything/StringCache-32              100000000              100.3 ns/op             0 B/op          0 allocs/op
BenchmarkEverything/UnsafeCache-32              135556000               87.31 ns/op            0 B/op          0 allocs/op
BenchmarkEverything/MapTTLCache-32              100000000              175.6 ns/op             0 B/op          0 allocs/op
BenchmarkEverything/RingBuffer-32               121507983               94.82 ns/op            0 B/op          0 allocs/op
BenchmarkEverything/ShardedRingBufferUpdater-32                 32976999               371.6 ns/op            18 B/op          0 allocs/op
BenchmarkEverything/KVMapCache-32                               90192560               199.9 ns/op             1 B/op          0 allocs/op
BenchmarkEverything/AnyCache-32                                 100000000              231.1 ns/op             8 B/op          1 allocs/op
BenchmarkKVListByPrefix-32                                       3167788              3720 ns/op             131 B/op          3 allocs/op
```

# Parallel benchmarks

~~I considered sharding cache to several buckets to ease lock contention, but after comparing test results with several cache libraries that have sharding, I do not see clear need for that. Maybe with 96 CPUs I would reconsider, but with 10 CPU I do not see a significant bottleneck in the mutex.~~

I implemented sharding anyway because why not. But it is a separate wrapper, so does not complicate existing codebase.

```shell
$ go test -benchtime=10s -benchmem -bench .
goos: linux
goarch: amd64
pkg: cache_bench
cpu: Intel(R) Xeon(R) Platinum 8358 CPU @ 2.60GHz
BenchmarkEverythingParallel/MapCache-32                 100000000              170.1 ns/op             0 B/op          0 allocs/op
BenchmarkEverythingParallel/MapTTLCache-32              90510988               198.9 ns/op             0 B/op          0 allocs/op
BenchmarkEverythingParallel/RingBuffer-32               85731428               196.8 ns/op             0 B/op          0 allocs/op
BenchmarkEverythingParallel/ShardedMapCache-32          273706551               43.51 ns/op            0 B/op          0 allocs/op
BenchmarkEverythingParallel/ShardedMapTTLCache-32               282491904               44.37 ns/op            0 B/op          0 allocs/op
BenchmarkEverythingParallel/ShardedRingBuffer-32                284756061               40.78 ns/op            0 B/op          0 allocs/op
BenchmarkEverythingParallel/github.com/Code-Hex/go-generics-cache-32            43165059               294.2 ns/op             7 B/op          0 allocs/op
BenchmarkEverythingParallel/github.com/Yiling-J/theine-go-32                    186976719               64.51 ns/op            0 B/op          0 allocs/op
BenchmarkEverythingParallel/github.com/jellydator/ttlcache-32                   29943469               376.3 ns/op            43 B/op          0 allocs/op
BenchmarkEverythingParallel/github.com/erni27/imcache-32                        531496862               23.35 ns/op           50 B/op          1 allocs/op
BenchmarkEverythingParallel/github.com/dgraph-io/ristretto-32                   100000000              108.5 ns/op            27 B/op          1 allocs/op
BenchmarkEverythingParallel/github.com/hashicorp/golang-lru/v2-32               43857675               307.1 ns/op             0 B/op          0 allocs/op
BenchmarkEverythingParallel/github.com/egregors/kesh-32                         33866130               428.7 ns/op            83 B/op          2 allocs/op
BenchmarkEverythingParallel/KVMapCache-32                                       43328151               401.2 ns/op           112 B/op          0 allocs/op
```

And now on 32 CPU machine we clearly see performance degradation due to lock contention. Sharded implementations are about 4 times faster.
Notice the Imcache result. Crazy fast! 😅

KV wrapper result is worse then other caches, but it is expected as it keeps key index allowing prefix search with deterministic order, that other caches do not allow. It updates trie structure on `Set` and does extra work to cleanup the key on `Del`.

```shell
$ go test -benchtime=10s -benchmem -bench .
goos: linux
goarch: amd64
pkg: cache_bench
cpu: Intel(R) Xeon(R) Platinum 8280 CPU @ 2.70GHz
BenchmarkEverythingParallel/MapCache-32         	64085875	       248.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkEverythingParallel/MapTTLCache-32      	58598002	       279.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkEverythingParallel/RingBuffer-32       	48229945	       315.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkEverythingParallel/ShardedMapCache-32  	234258486	        53.16 ns/op	       0 B/op	       0 allocs/op
BenchmarkEverythingParallel/ShardedMapTTLCache-32         	231177732	        53.63 ns/op	       0 B/op	       0 allocs/op
BenchmarkEverythingParallel/ShardedRingBuffer-32          	236979438	        48.98 ns/op	       0 B/op	       0 allocs/op
BenchmarkEverythingParallel/github.com/Code-Hex/go-generics-cache-32         	39842918	       345.9 ns/op	       7 B/op	       0 allocs/op
BenchmarkEverythingParallel/github.com/Yiling-J/theine-go-32                 	150612642	        81.82 ns/op	       0 B/op	       0 allocs/op
BenchmarkEverythingParallel/github.com/jellydator/ttlcache-32                	29333647	       433.9 ns/op	      43 B/op	       0 allocs/op
BenchmarkEverythingParallel/github.com/erni27/imcache-32                     	345577933	        35.63 ns/op	      50 B/op	       1 allocs/op
BenchmarkEverythingParallel/github.com/dgraph-io/ristretto-32                	83293519	       142.1 ns/op	      27 B/op	       1 allocs/op
BenchmarkEverythingParallel/github.com/hashicorp/golang-lru/v2-32            	35763888	       378.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkEverythingParallel/github.com/egregors/kesh-32                      	25860772	       524.1 ns/op	      84 B/op	       2 allocs/op
BenchmarkEverythingParallel/KVMapCache-32                                    	33802629	       478.4 ns/op	     109 B/op	       0 allocs/op
PASS
```

Concurrent comparison benchmark is located in a [separate repository](https://github.com/C-Pro/cache-benchmarks) to avoid pulling unnecessary dependencies in the library.
