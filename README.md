# Geche (generic cache)

Benchmarking four simple cache implementations shows that generic cache (`MapCache`) is faster than cache that uses an empty interface for values (`AnyCache`) (244ns vs 429ns), but slower than implementations that use concrete types (`StringCache`) at 236ns and skip on thread safety (`UnsafeCache`) at 429ns.

```shell
$ GOGC=off go test -count=3 -benchtime=1s -bench . .
goos: linux
goarch: amd64
pkg: geche
cpu: Intel(R) Core(TM) i5-8250U CPU @ 1.60GHz
BenchmarkSet/MapCache-8                  4631028               357.8 ns/op
BenchmarkSet/MapCache-8                  5367727               244.0 ns/op
BenchmarkSet/MapCache-8                  5321649               244.0 ns/op
BenchmarkSet/StringCache-8               3583323               359.6 ns/op
BenchmarkSet/StringCache-8               5146477               235.6 ns/op
BenchmarkSet/StringCache-8               5360342               235.8 ns/op
BenchmarkSet/UnsafeCache-8               3945006               366.1 ns/op
BenchmarkSet/UnsafeCache-8               6308018               209.5 ns/op
BenchmarkSet/UnsafeCache-8               6351516               213.3 ns/op
BenchmarkSet/AnyCache-8                  2859843               429.1 ns/op
BenchmarkSet/AnyCache-8                  2707725               438.1 ns/op
BenchmarkSet/AnyCache-8                  2651115               441.7 ns/op
PASS
ok      geche   31.351s
```
