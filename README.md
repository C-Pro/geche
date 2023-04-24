# Geche (generic cache)

[![Test](https://github.com/C-Pro/geche/actions/workflows/build.yml/badge.svg)](https://github.com/C-Pro/geche/actions/workflows/build.yml)

Collection of generic cache implementations in Go for cases when you don't want a lot of `interface{}` values in your process memory.
Implementations are as simple as possible to be predictable in max latency, memory allocation and concurrency impact (writes lock reads and are serialized with other writes).

* `MapCache` is a very simple map-based thread-safe cache, that is not limited from growing. Can be used when you have relatively small number of distinct keys that does not grow significantly, and you do not need the values to expire automatically. E.g. if your keys are country codes, timezones etc, this cache type is ok to use.
* `MapTTLCache` is map-based thread-safe cache with support for TTL (values automatically expire). If you don't want to read value from cache that is older then some threshold (e.g. 1 sec), you set this TTL when initializing the cache object and obsolete rows will be removed from cache automatically.
* `RingBuffer` is a predefined size cache that allocates all memory from the start and will not grow above it. It keeps constant size by overwriting the oldest values in the cache with new ones. Use this cache when you need speed and fixed memory footprint, and your key cardinality is predictable (or you are ok with having cache misses if cardinality suddenly grows above your cache size).


## Benchmarks

Test suite contains a couple of benchmarks to compare the speed difference between old-school generic implementation using `interface{}` or `any` to hold cache values versus using generics.

TL/DR: generics are faster than `interface{}` but slower than hardcoded type implementation. Ring buffer is 2x+ faster then map-based TTL cache.

Benchmarking four simple cache implementations shows that generic cache (`MapCache`) is faster than cache that uses an empty interface to store any type of values (`AnyCache`), but slower than implementations that use concrete types (`StringCache`) and skip on thread safety (`UnsafeCache`).
Generic `MapTTLCache` is on par with `AnyCache` but it is to be expected as it does more work keeping linked list for fast invalidation. `RingBuffer` performs the best because all the space it needs is preallocated during the initialization, and actual cache size is limited.

There are two types of benchmarks:
* `BenchmarkSet` only times the `Set` operation that allocates all the memory, and usually is the most resource intensive.
* `BenchmarkEverything` repeatedly does one of three operations (Get/Set/Del). The probability for each type of operation to be executed is 0.9/0.05/0.05 respectively. Each operation is executed on randomly generated key, there are totally 1 million distinct keys, so total cache size will be limited too.

Note that `stringCache`, `unsafeCache`, `anyCache` implementations are unexported. These implementations exist only to compare Go generic implementation with other approaches.

The results below are not to be treated as absolute values. Actual cache operation latency will depend on many variables such as CPU speed, key cardinality, number of concurrent operations, whether the allocation happen during the operation or underlying structure already has the allocated space and so on.

```shell
$ go test -count=1 -benchtime=5s -benchmem -bench . .
goos: linux
goarch: amd64
pkg: geche
cpu: Intel(R) Core(TM) i5-8250U CPU @ 1.60GHz
BenchmarkSet/MapCache-8         24601024               245.0 ns/op             7 B/op          0 allocs/op
BenchmarkSet/StringCache-8              24591309               242.6 ns/op             7 B/op          0 allocs/op
BenchmarkSet/UnsafeCache-8              28177236               211.6 ns/op             7 B/op          0 allocs/op
BenchmarkSet/MapTTLCache-8              11912029               501.8 ns/op             7 B/op          0 allocs/op
BenchmarkSet/RingBuffer-8               33171223               186.5 ns/op            10 B/op          1 allocs/op
BenchmarkSet/AnyCache-8                 24146725               248.4 ns/op            14 B/op          1 allocs/op
BenchmarkEverything/MapCache-8          32745301               315.2 ns/op            10 B/op          1 allocs/op
BenchmarkEverything/StringCache-8       42918726               323.4 ns/op             9 B/op          1 allocs/op
BenchmarkEverything/UnsafeCache-8       47435004               312.6 ns/op             9 B/op          1 allocs/op
BenchmarkEverything/MapTTLCache-8       36476836               413.7 ns/op            13 B/op          1 allocs/op
BenchmarkEverything/RingBuffer-8        41306090               162.0 ns/op             8 B/op          1 allocs/op
BenchmarkEverything/AnyCache-8          43765518               300.4 ns/op             9 B/op          1 allocs/op
PASS
ok      geche   132.965s
```
