# Geche (generic cache)

[![Workflow status](https://github.com/C-Pro/geche/actions/workflows/test.yml/badge.svg)](https://github.com/C-Pro/geche/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/c-pro/geche)](https://goreportcard.com/report/github.com/c-pro/geche)
[![GoDoc](https://pkg.go.dev/badge/mod/github.com/c-pro/geche)](https://pkg.go.dev/mod/github.com/c-pro/geche)
[![Coverage Status](https://codecov.io/gh/c-pro/geche/branch/main/graph/badge.svg)](https://codecov.io/gh/c-pro/geche)

Collection of generic cache implementations in Go focused on simplicity. No clever tricks, no dependencies, no gazillion of configuration options.
Implementations are as simple as possible to be predictable in max latency, memory allocation and concurrency impact (writes lock reads and are serialized with other writes).

* `MapCache` is a very simple map-based thread-safe cache, that is not limited from growing. Can be used when you have relatively small number of distinct keys that does not grow significantly, and you do not need the values to expire automatically. E.g. if your keys are country codes, timezones etc, this cache type is ok to use.
* `MapTTLCache` is map-based thread-safe cache with support for TTL (values automatically expire). If you don't want to read value from cache that is older then some threshold (e.g. 1 sec), you set this TTL when initializing the cache object and obsolete rows will be removed from cache automatically.
* `RingBuffer` is a predefined size cache that allocates all memory from the start and will not grow above it. It keeps constant size by overwriting the oldest values in the cache with new ones. Use this cache when you need speed and fixed memory footprint, and your key cardinality is predictable (or you are ok with having cache misses if cardinality suddenly grows above your cache size).

## Example

Interface is very simple with three methods: `Set`, `Get`, `Del`. Here's a quick example for a ring buffer holding 10k records.

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

    fmt.Println(v)
}
```

If you intend to use cache in *higlhy* concurrent manner (16+ cores and 100k+ RPS). It may make sense to shard it.
To shard the cache you need to wrap it using `NewSharded`:

```go
func main() {
    // Create sharded TTL cache with number of shards defined automatically
    // based on number of available CPUs.
    c := NewSharded[string](
					func() Geche[string, string] {
                        return NewMapTTLCache[string, string](ctx, time.Second, time.Second)
                    },
					0,
					&StringMapper{0},
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

### CacheUpdater

Another useful wrapper is the `CacheUpdater`. It implements a common scenario, when on cache miss some function is called to get the value from external resource (database, API, some lengthy calculation).
This scenario sounds deceptively simple, but with straightforward implementation can lead to nasty problems like [cache stampede](https://en.wikipedia.org/wiki/Cache_stampede).
To avoid these types of problems `CacheUpdater` has two limiting mechanisms:

* Pool size limits number of keys that can be updated concurrently. E.g. if you set pool size of 10, your cache update will never run more then 10 simultaneous queries to get the value that was not fount in the cache.
* In flight mechanism does not allow to call update function to get the value for the key if the update function for this key is already running. E.g. if suddenly 10 requests for the same key hit the cache and miss (key not in the cache), only first will trigger the update, others will just wait for the result.

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

## Benchmarks

Test suite contains a couple of benchmarks to compare the speed difference between old-school generic implementation using `interface{}` or `any` to hold cache values versus using generics.

TL/DR: generics are faster than `interface{}` but slower than hardcoded type implementation. Ring buffer is 2x+ faster then map-based TTL cache.

There are two types of benchmarks:
* `BenchmarkSet` only times the `Set` operation that allocates all the memory, and usually is the most resource intensive.
* `BenchmarkEverything` repeatedly does one of three operations (Get/Set/Del). The probability for each type of operation to be executed is 0.9/0.05/0.05 respectively. Each operation is executed on randomly generated key, there are totally 1 million distinct keys, so total cache size will be limited too.

Benchmarking four simple cache implementations shows that generic cache (`MapCache`) is faster than cache that uses an empty interface to store any type of values (`AnyCache`), but slower than implementations that use concrete types (`StringCache`) and skip on thread safety (`UnsafeCache`).
Generic `MapTTLCache` is on par with `AnyCache` but it is to be expected as it does more work keeping linked list for fast invalidation. `RingBuffer` performs the best because all the space it needs is preallocated during the initialization, and actual cache size is limited.

Note that `stringCache`, `unsafeCache`, `anyCache` implementations are unexported. These implementations exist only to compare Go generic implementation with other approaches.

The results below are not to be treated as absolute values. Actual cache operation latency will depend on many variables such as CPU speed, key cardinality, number of concurrent operations, whether the allocation happen during the operation or underlying structure already has the allocated space and so on.

```shell
 $ go test -bench . -benchmem -benchtime=30s
goos: darwin
goarch: arm64
pkg: github.com/c-pro/geche
BenchmarkSet/MapCache-10  	238400880	       151.4 ns/op	       7 B/op	       0 allocs/op
BenchmarkSet/StringCache-10         	240488745	       149.7 ns/op	       7 B/op	       0 allocs/op
BenchmarkSet/UnsafeCache-10         	348324978	       102.9 ns/op	       7 B/op	       0 allocs/op
BenchmarkSet/MapTTLCache-10         	89931351	       338.2 ns/op	       7 B/op	       0 allocs/op
BenchmarkSet/RingBuffer-10          	215545424	       166.1 ns/op	       7 B/op	       0 allocs/op
BenchmarkSet/AnyCache-10            	241277830	       149.4 ns/op	       8 B/op	       1 allocs/op
BenchmarkEverything/MapCache-10     	333596707	       110.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkEverything/StringCache-10  	327069014	       112.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkEverything/UnsafeCache-10  	535376823	        68.41 ns/op	       0 B/op	       0 allocs/op
BenchmarkEverything/MapTTLCache-10  	222688748	       166.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkEverything/RingBuffer-10   	671402931	        53.76 ns/op	       0 B/op	       0 allocs/op
BenchmarkEverything/AnyCache-10     	195838436	       195.8 ns/op	       8 B/op	       1 allocs/op
PASS
ok  	github.com/c-pro/geche	577.123s
```

# Parallel benchmarks

~~I considered sharding cache to several buckets to ease lock contention, but after comparing test results with several cache libraries that have sharding, I do not see clear need for that. Maybe with 96 CPUs I would reconsider, but with 10 CPU I do not see a significant bottleneck in the mutex.~~

I implemented sharding anyway because why not. But it is a separate wrapper, so does not complicate existing codebase.

```shell
$ go test -benchtime=30s -benchmem -bench .
goos: darwin
goarch: arm64
pkg: cache_bench
BenchmarkEverythingParallel/MapCache-10                 332130052              133.3 ns/op             0 B/op          0 allocs/op
BenchmarkEverythingParallel/MapTTLCache-10              234690624              205.4 ns/op             0 B/op          0 allocs/op
BenchmarkEverythingParallel/RingBuffer-10               441694302               86.82 ns/op            0 B/op          0 allocs/op
BenchmarkEverythingParallel/github.com/Code-Hex/go-generics-cache-10            191366336              198.8 ns/op             7 B/op          0 allocs/op
BenchmarkEverythingParallel/github.com/Yiling-J/theine-go-10                    367538067              100.7 ns/op             0 B/op          0 allocs/op
BenchmarkEverythingParallel/github.com/jellydator/ttlcache-10                   136785907              262.4 ns/op            43 B/op          0 allocs/op
BenchmarkEverythingParallel/github.com/erni27/imcache-10                        226084180              179.2 ns/op             2 B/op          0 allocs/op
BenchmarkEverythingParallel/github.com/dgraph-io/ristretto-10                   466729495               80.03 ns/op           30 B/op          1 allocs/op
BenchmarkEverythingParallel/github.com/hashicorp/golang-lru/v2-10               193697901              216.5 ns/op             0 B/op          0 allocs/op
PASS
ok      cache_bench     496.390s
```

And now on 96 CPU machine we clearly see performance degradation due to lock contention. Sharded implementations are about 4 times faster.
Notice the Imcache result. Is too good to be true 😅

```
go test -bench . -benchtime=30s
goos: linux
goarch: amd64
pkg: cache_bench
cpu: Intel(R) Xeon(R) Platinum 8275CL CPU @ 3.00GHz
BenchmarkEverythingParallel/MapCache-96         	170261677	       237.8 ns/op
BenchmarkEverythingParallel/MapTTLCache-96      	142511408	       272.9 ns/op
BenchmarkEverythingParallel/RingBuffer-96       	173581318	       230.8 ns/op
BenchmarkEverythingParallel/ShardedMapCache-96  	572037444	        56.76 ns/op
BenchmarkEverythingParallel/ShardedMapTTLCache-96         	607333974	        58.35 ns/op
BenchmarkEverythingParallel/ShardedRingBuffer-96          	599080330	        52.26 ns/op
BenchmarkEverythingParallel/github.com/Code-Hex/go-generics-cache-96         	148365510	       263.4 ns/op
BenchmarkEverythingParallel/github.com/Yiling-J/theine-go-96                 	289192544	       122.9 ns/op
BenchmarkEverythingParallel/github.com/jellydator/ttlcache-96                	101329562	       383.6 ns/op
BenchmarkEverythingParallel/github.com/erni27/imcache-96                     	1000000000	        12.09 ns/op
BenchmarkEverythingParallel/github.com/dgraph-io/ristretto-96                	322868853	       111.6 ns/op
BenchmarkEverythingParallel/github.com/hashicorp/golang-lru/v2-96            	145826460	       259.8 ns/op
PASS
ok  	cache_bench	608.617s
```


Concurrent comparison benchmark is located in a [separate repository](https://github.com/C-Pro/cache-benchmarks) to avoid pulling unnecessary dependencies in the library.
