
# Lac - Linear Allocator for Golang

## Goal
Speed up the memory allocation and improve the GC performance, especially for dynamic-memory-heavy applications.

NOTE: current version need go1.18+.

## Potential Use cases
1. A large amount of memory never needs to be released. (global configs, read-only assets like navmesh)
2. Massive temporary objects with deterministic lifetime. (protobuf objects send to network)

## Highlights
Linear allocator:

1. Mush faster on memory allocating. An allocation is just a pointer advancement internally.
2. Can greatly reduce the object marking pressure of GC. Lac is just a few byte arrays internally.
3. More general. Lac can allocate various types of objects.
4. Much simpler and faster on reclaiming memories. No need to manually release every object back but just reset the allocation cursor.
5. Much cheaper. Lac reuse memory chunks among each other via chunk pool. 
6. Memory efficient. Memories are more compact, CPU cache-friendly.
7. Allows build-in allocated objects to be attached to the Lac allocated objects. 
8. Support concurrency.
9. Provide protobuf2 like APIs.


## Limitations
1. Never store pointers to build-in allocated objects into Lac allocated objects **directly**. (There's a debug mode for checking external pointers)
2. Never store or use pointers to Lac allocated objects after the allocator is released. (In debug mode, the allocator traverses the objects and obfuscate the pointers to make any attempting usage panic)
3. Map memory can't use Lac and fallback to build-in allocator.


# Pros over v1.20 arena
1. Faster(see benchmark results below).
2. Support concurrency.
3. Slice append can utilize Lac as well.
4. Support debugging mode.
5. Provide protobuf2 like APIs.
6. Completely pointer free (no pointer bitmap initializing, no GC marking, etc).
7. Do not zero slices by default.

## Usage

```go
type PbItem struct {
	Id     *int
	Price  *int
	Class  *int
	Name   *string
	Active *bool
}

type PbData struct {
	Age   *int
	Items []*PbItem
	InUse *PbItem
}

func main() {	
	ac := lac.Get()
	defer ac.Release()
	
	d := lac.New[PbData](ac)
	d.Age = ac.Int(11)

	n := 3
	for i := 0; i < n; i++ {
		item := lac.New[PbItem](ac)
		item.Id = ac.Int(i + 1)
		item.Active = ac.Bool(true)
		item.Price = ac.Int(100 + i)
		item.Class = ac.Int(3 + i)
		item.Name = ac.String("name")

		d.Items = Append(ac, d.Items, item)
	}
}
```

## Benchmarks
Results from benchmark tests:

### Linux
- go test -bench . -benchmem


```
goos: linux
goarch: amd64
pkg: oops/lib/linear_ac/lac
cpu: Intel(R) Core(TM) i5-8500 CPU @ 3.00GHz
BenchmarkNew-6                   2655375               474.3 ns/op           185 B/op          0 allocs/op
BenchmarkNewFrom-6               3093921               476.7 ns/op           147 B/op          0 allocs/op
Benchmark_RawMalloc-6           10047429               106.1 ns/op            88 B/op          5 allocs/op
Benchmark_LacMalloc-6           14244061               115.0 ns/op           128 B/op          0 allocs/op
Benchmark_RawMallocLarge2-6        27825             40892 ns/op           27496 B/op       1656 allocs/op
Benchmark_LacMallocLarge2-6        51686             24068 ns/op           39583 B/op          0 allocs/op
PASS
```

- go test -bench . -tags=goexperiment.arenas -benchmem

(A simple test shows allocation performance compared with v1.20 arena)

```
goos: linux
goarch: amd64
pkg: oops/lib/linear_ac/lac
cpu: Intel(R) Core(TM) i5-8500 CPU @ 3.00GHz
BenchmarkNew-6                   2663191               459.9 ns/op           184 B/op          0 allocs/op
BenchmarkNewFrom-6               2989390               473.0 ns/op           149 B/op          0 allocs/op
Benchmark_RawMalloc-6           10412677               105.6 ns/op            88 B/op          5 allocs/op
Benchmark_LacMalloc-6           14276998               114.6 ns/op           128 B/op          0 allocs/op
Benchmark_RawMallocLarge2-6        26142             40217 ns/op           27496 B/op       1656 allocs/op
Benchmark_LacMallocLarge2-6        50194             24815 ns/op           39590 B/op          0 allocs/op
Benchmark_RawMallocSmall-6      10188718               178.4 ns/op            88 B/op          5 allocs/op
Benchmark_LacMallocSmall-6      13542043                90.15 ns/op          128 B/op          0 allocs/op
Benchmark_ArenaMallocSmall-6    12588194                94.69 ns/op           87 B/op          0 allocs/op
Benchmark_RawMallocLarge-6         31762             35838 ns/op           27496 B/op       1656 allocs/op
Benchmark_LacMallocLarge-6         50337             26381 ns/op           39582 B/op          0 allocs/op
Benchmark_ArenaMallocLarge-6       36783             30900 ns/op           26634 B/op         45 allocs/op

```

### Windows
- go test -bench . -benchmem

```
goos: windows
goarch: amd64
pkg: linear_ac/lac
cpu: Intel(R) Core(TM) i7-10510U CPU @ 1.80GHz
BenchmarkNew-8                   2388756               569.3 ns/op           197 B/op          0 allocs/op
BenchmarkNewFrom-8               2584728               552.0 ns/op           138 B/op          0 allocs/op
Benchmark_RawMalloc-8            9610574               125.9 ns/op            88 B/op          5 allocs/op
Benchmark_LacMalloc-8           10282324               114.4 ns/op           128 B/op          0 allocs/op
Benchmark_RawMallocLarge2-8        23122             47686 ns/op           27496 B/op       1656 allocs/op
Benchmark_LacMallocLarge2-8        42115             27730 ns/op           39590 B/op          0 allocs/op

```
- go test -bench . -tags='goexperiment.arenas' -benchmem
```
goos: windows
goarch: amd64
pkg: linear_ac/lac
cpu: Intel(R) Core(TM) i7-10510U CPU @ 1.80GHz
BenchmarkNew-8                   2632837               499.3 ns/op           186 B/op          0 allocs/op
BenchmarkNewFrom-8               2989316               493.2 ns/op           149 B/op          0 allocs/op
Benchmark_RawMalloc-8            9782223               123.4 ns/op            88 B/op          5 allocs/op
Benchmark_LacMalloc-8           10231971               115.1 ns/op           128 B/op          0 allocs/op
Benchmark_RawMallocLarge2-8        27981             37875 ns/op           27496 B/op       1656 allocs/op
Benchmark_LacMallocLarge2-8        41592             29860 ns/op           39584 B/op          0 allocs/op
Benchmark_RawMallocSmall-8       7730782               147.3 ns/op            88 B/op          5 allocs/op
Benchmark_LacMallocSmall-8      12129996                92.38 ns/op          128 B/op          0 allocs/op
Benchmark_ArenaMallocSmall-8     8912542               134.0 ns/op            88 B/op          0 allocs/op
Benchmark_RawMallocLarge-8         33466             39580 ns/op           27496 B/op       1656 allocs/op
Benchmark_LacMallocLarge-8         41583             27847 ns/op           39592 B/op          0 allocs/op
Benchmark_ArenaMallocLarge-8       25872             49512 ns/op           26549 B/op         45 allocs/op

```