# Geche (generic cache)

[![Test](https://github.com/C-Pro/geche/actions/workflows/build.yml/badge.svg)](https://github.com/C-Pro/geche/actions/workflows/build.yml)

Collection of generic cache implementations for cases when you don't want a lot of `interface{}` values in your process memory.

* `MapCache` is a very simple map-based thread-safe cache.
* `MapTTLCache` is map-based thread-safe cache with support for TTL (values automatically expire).

## Set benchmark

This is a simple benchmark the purpose of which is to understand the speed difference between old-school generic implementation using `interface{}` or `any` to hold cache values versus using generics.

TL/DR: generics are faster than `interface{}` but slower than hardcoded type implementation.

Benchmarking four simple cache implementations shows that generic cache (`MapCache`) is faster than cache that uses an empty interface for values (`AnyCache`) (244ns vs 429ns), but slower than implementations that use concrete types (`StringCache`) at 236ns and skip on thread safety (`UnsafeCache`) at 210ns.
Generic `MapTTLCache` is on par with `AnyCache` but it is to be expected as it does more work keeping linked list for fast invalidation.

```shell
$ GOGC=off go test -count=3 -benchtime=1s -bench . .
goos: linux
goarch: amd64
pkg: geche
cpu: Intel(R) Core(TM) i5-8250U CPU @ 1.60GHz
BenchmarkSet/MapCache-8                  4255777               473.4 ns/op
BenchmarkSet/MapCache-8                  4439835               291.1 ns/op
BenchmarkSet/MapCache-8                  4596613               289.4 ns/op
BenchmarkSet/StringCache-8               2318277               437.1 ns/op
BenchmarkSet/StringCache-8               4610161               457.6 ns/op
BenchmarkSet/StringCache-8               4484840               281.8 ns/op
BenchmarkSet/UnsafeCache-8               2525283               398.1 ns/op
BenchmarkSet/UnsafeCache-8               5006996               419.0 ns/op
BenchmarkSet/UnsafeCache-8               5198524               265.2 ns/op
BenchmarkSet/MapTTLCache-8               2424573               736.7 ns/op
BenchmarkSet/MapTTLCache-8               2590296               520.2 ns/op
BenchmarkSet/MapTTLCache-8               2538258               522.9 ns/op
BenchmarkSet/AnyCache-8                  2526578               520.9 ns/op
BenchmarkSet/AnyCache-8                  2427645               523.8 ns/op
BenchmarkSet/AnyCache-8                  2410173               520.5 ns/op
PASS
ok      geche   41.525s
```
