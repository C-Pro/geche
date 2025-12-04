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
* `KVCache` is a specialized cache designed for efficient prefix-based key lookups. It uses a trie data structure to store keys, enabling lexicographical ordering and fast retrieval of all values whose keys start with a given prefix.

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

`Updater` provides `ListByPrefix` function, but it can be used only if underlying cache supports it.
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

Benchmarks are designed to compare basic operations of different cache implementations in this library.

```shell
 $ go test -bench=. -benchmem -benchtime=10s .
goos: linux
goarch: amd64
pkg: github.com/c-pro/geche
cpu: AMD Ryzen 7 PRO 8840U w/ Radeon 780M Graphics
BenchmarkSet/MapCache-16                65541076               182.1 ns/op             0 B/op          0 allocs/op
BenchmarkSet/MapTTLCache-16             19751806               754.7 ns/op            10 B/op          0 allocs/op
BenchmarkSet/RingBuffer-16              51921265               365.9 ns/op             0 B/op          0 allocs/op
BenchmarkSet/KVMapCache-16               3876873              3461 ns/op             804 B/op         11 allocs/op
BenchmarkSet/KVCache-16                 14983084              1025 ns/op              54 B/op          1 allocs/op
BenchmarkSetIfPresentOnlyHits/MapCache-16               79179759               187.6 ns/op             0 B/op          0 allocs/op
BenchmarkSetIfPresentOnlyHits/MapTTLCache-16            37620368               371.8 ns/op             0 B/op          0 allocs/op
BenchmarkSetIfPresentOnlyHits/RingBuffer-16             100000000              110.3 ns/op             0 B/op          0 allocs/op
BenchmarkSetIfPresentOnlyHits/KVMapCache-16             39745081               345.1 ns/op             8 B/op          1 allocs/op
BenchmarkSetIfPresentOnlyMisses/MapCache-16             786898237               15.04 ns/op            0 B/op          0 allocs/op
BenchmarkSetIfPresentOnlyMisses/MapTTLCache-16          648632726               18.43 ns/op            0 B/op          0 allocs/op
BenchmarkSetIfPresentOnlyMisses/RingBuffer-16           746030799               15.92 ns/op            0 B/op          0 allocs/op
BenchmarkSetIfPresentOnlyMisses/KVMapCache-16           625973469               19.00 ns/op            0 B/op          0 allocs/op
BenchmarkSetIfPresentOnlyMisses/KVCache-16              972807471               12.02 ns/op            0 B/op          0 allocs/op
BenchmarkGetHit/MapCache-16                             100000000              104.8 ns/op             0 B/op          0 allocs/op
BenchmarkGetHit/MapTTLCache-16                          57810127               261.9 ns/op             0 B/op          0 allocs/op
BenchmarkGetHit/RingBuffer-16                           121727826               98.63 ns/op            0 B/op          0 allocs/op
BenchmarkGetHit/KVMapCache-16                           100000000              106.3 ns/op             0 B/op          0 allocs/op
BenchmarkGetHit/KVCache-16                              158599485               78.32 ns/op            0 B/op          0 allocs/op
BenchmarkGetMiss/MapCache-16                            1000000000              11.01 ns/op            0 B/op          0 allocs/op
BenchmarkGetMiss/MapTTLCache-16                         749231084               15.85 ns/op            0 B/op          0 allocs/op
BenchmarkGetMiss/RingBuffer-16                          676585886               17.73 ns/op            0 B/op          0 allocs/op
BenchmarkGetMiss/KVMapCache-16                          1000000000              11.64 ns/op            0 B/op          0 allocs/op
BenchmarkGetMiss/KVCache-16                             297815424               39.80 ns/op            0 B/op          0 allocs/op
BenchmarkDelHit/MapCache-16                             1000000000              10.84 ns/op            0 B/op          0 allocs/op
BenchmarkDelHit/MapTTLCache-16                          756901813               14.37 ns/op            0 B/op          0 allocs/op
BenchmarkDelHit/RingBuffer-16                           1000000000              10.28 ns/op            0 B/op          0 allocs/op
BenchmarkDelHit/KVMapCache-16                           358719861               28.27 ns/op            1 B/op          0 allocs/op
BenchmarkDelHit/KVCache-16                              366528763               31.60 ns/op           17 B/op          1 allocs/op
BenchmarkDelMiss/MapCache-16                            792498559               15.05 ns/op            0 B/op          0 allocs/op
BenchmarkDelMiss/MapTTLCache-16                         735312480               16.18 ns/op            0 B/op          0 allocs/op
BenchmarkDelMiss/RingBuffer-16                          364969610               32.75 ns/op            0 B/op          0 allocs/op
BenchmarkDelMiss/KVMapCache-16                          78108807               153.3 ns/op            64 B/op          1 allocs/op
BenchmarkDelMiss/KVCache-16                             49184259               233.0 ns/op           352 B/op          4 allocs/op
BenchmarkEverything/MapCache-16                         67129406               175.8 ns/op             0 B/op          0 allocs/op
BenchmarkEverything/MapTTLCache-16                      24496364               650.0 ns/op             8 B/op          0 allocs/op
BenchmarkEverything/RingBuffer-16                       60798320               304.7 ns/op             0 B/op          0 allocs/op
BenchmarkEverything/ShardedRingBufferUpdater-16         44071453               427.5 ns/op            19 B/op          0 allocs/op
BenchmarkEverything/KVMapCache-16                        5465926              2695 ns/op             745 B/op         10 allocs/op
BenchmarkEverything/KVCache-16                          17036607              1020 ns/op              60 B/op          0 allocs/op
BenchmarkEverything/LockerMapCache-16                   48808772               209.0 ns/op             1 B/op          0 allocs/op
BenchmarkKVListByPrefix-16                               3052804              3905 ns/op            1008 B/op         15 allocs/op
BenchmarkKVCacheListByPrefix-16                          8222977              1438 ns/op             580 B/op          8 allocs/op
PASS
ok      github.com/c-pro/geche  956.205s
```

# Parallel benchmarks

~~I considered sharding cache to several buckets to ease lock contention, but after comparing test results with several cache libraries that have sharding, I do not see clear need for that. Maybe with 96 CPUs I would reconsider, but with 10 CPU I do not see a significant bottleneck in the mutex.~~

I implemented sharding anyway because why not. But it is a separate wrapper, so does not complicate existing codebase.

```shell
go test -benchtime=10s -benchmem -bench .
goos: linux
goarch: amd64
pkg: cache_bench
cpu: AMD Ryzen 7 PRO 8840U w/ Radeon 780M Graphics
BenchmarkEverythingParallel/MapCache-16         	100000000	       173.9 ns/op	       1 B/op	       0 allocs/op
BenchmarkEverythingParallel/MapTTLCache-16      	65010415	       382.5 ns/op	       2 B/op	       0 allocs/op
BenchmarkEverythingParallel/RingBuffer-16       	100000000	       225.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkEverythingParallel/ShardedMapCache-16  	198813898	        56.77 ns/op	       0 B/op	       0 allocs/op
BenchmarkEverythingParallel/ShardedMapTTLCache-16         	122482419	        97.60 ns/op	       0 B/op	       0 allocs/op
BenchmarkEverythingParallel/ShardedRingBuffer-16          	188570131	        63.23 ns/op	       0 B/op	       0 allocs/op
BenchmarkEverythingParallel/github.com/Code-Hex/go-generics-cache-16         	55945956	       474.2 ns/op	      91 B/op	       2 allocs/op
BenchmarkEverythingParallel/github.com/Yiling-J/theine-go-16                 	71100289	       172.2 ns/op	       1 B/op	       0 allocs/op
BenchmarkEverythingParallel/github.com/jellydator/ttlcache-16                	58265994	       924.8 ns/op	      30 B/op	       0 allocs/op
BenchmarkEverythingParallel/github.com/erni27/imcache-16                     	236973852	        45.84 ns/op	      33 B/op	       0 allocs/op
BenchmarkEverythingParallel/github.com/dgraph-io/ristretto-16                	88618468	       141.9 ns/op	      94 B/op	       2 allocs/op
BenchmarkEverythingParallel/github.com/hashicorp/golang-lru/v2-16            	78454165	       399.1 ns/op	       2 B/op	       0 allocs/op
BenchmarkEverythingParallel/github.com/egregors/kesh-16                      	68416022	       337.8 ns/op	       7 B/op	       0 allocs/op
BenchmarkEverythingParallel/KVMapCache-16                                    	 7607014	      2050 ns/op	     254 B/op	       3 allocs/op
BenchmarkEverythingParallel/KVCache-16                                       	52397652	       902.8 ns/op	      53 B/op	       0 allocs/op
BenchmarkEverythingParallel/ShardedKVCache-16                                	100000000	       150.9 ns/op	      45 B/op	       0 allocs/op
PASS
ok  	cache_bench	390.130s
```

KV and KVCache results are worse then other caches, but it is expected as it keeps key index allowing prefix search with deterministic order, that other caches do not allow. It updates trie structure on `Set` and does extra work to cleanup the key on `Del`.

Concurrent comparison benchmark is located in a [separate repository](https://github.com/C-Pro/cache-benchmarks) to avoid pulling unnecessary dependencies in the library.
