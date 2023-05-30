
# Lac - Linear Allocator for Golang

## Goal
Speed up the memory allocation and improve the GC performance, especially for dynamic-memory-heavy applications.

NOTE: need go1.18+.

## Potential Use cases
1. A large amount of memory never needs to be released. (global configs, read-only assets like navmesh)
2. Massive temporary objects with deterministic lifetime. (protobuf objects sent to network)

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
9. Provide protobuf2 style APIs.


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
	ac := acPool.Get()
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
BenchmarkNew-6                             65127            118814 ns/op             167 B/op          0 allocs/op
BenchmarkNewFrom-6                         64368            117673 ns/op             217 B/op          1 allocs/op
Benchmark_RawMalloc-6                    9584208             124.6 ns/op              88 B/op          5 allocs/op
Benchmark_LacMalloc-6                   17156577             68.20 ns/op               0 B/op          0 allocs/op
Benchmark_LacMallocMt-6                 17331070             69.52 ns/op               0 B/op          0 allocs/op
Benchmark_RawMallocLarge2-6                39134             35891 ns/op           27496 B/op       1656 allocs/op
Benchmark_LacMallocLarge2-6               115216             14820 ns/op            9135 B/op          0 allocs/op
Benchmark_LacMallocLarge2Mt-6              62988             19065 ns/op               0 B/op          0 allocs/op

```

- go test -bench . -tags=goexperiment.arenas -benchmem

(A simple test shows allocation performance compared with v1.20 arena)

```
goos: linux
goarch: amd64
pkg: oops/lib/linear_ac/lac
cpu: Intel(R) Core(TM) i5-8500 CPU @ 3.00GHz
BenchmarkNew-6                             65473            118687 ns/op             166 B/op          0 allocs/op
BenchmarkNewFrom-6                         65342            118379 ns/op             215 B/op          1 allocs/op
Benchmark_RawMalloc-6                    9832786             122.7 ns/op              88 B/op          5 allocs/op
Benchmark_LacMalloc-6                   16667865             69.94 ns/op               0 B/op          0 allocs/op
Benchmark_LacMallocMt-6                 17386522             70.15 ns/op               0 B/op          0 allocs/op
Benchmark_RawMallocLarge2-6                40267             35076 ns/op           27496 B/op       1656 allocs/op
Benchmark_LacMallocLarge2-6               109783             14897 ns/op            8239 B/op          0 allocs/op
Benchmark_LacMallocLarge2Mt-6              63415             18895 ns/op               0 B/op          0 allocs/op
Benchmark_RawMallocSmall-6               9126696             128.5 ns/op              88 B/op          5 allocs/op
Benchmark_LacMallocSmall-6              30242865             43.19 ns/op               0 B/op          0 allocs/op
Benchmark_ArenaMallocSmall-6            12157006             160.3 ns/op              88 B/op          0 allocs/op
Benchmark_RawMallocLarge-6                 43189             34958 ns/op           27496 B/op       1656 allocs/op
Benchmark_LacMallocLarge-6                 93844             12481 ns/op           27267 B/op          0 allocs/op
Benchmark_ArenaMallocLarge-6               35060             31891 ns/op           26637 B/op         45 allocs/op
```

### Windows
- go test -bench . -benchmem

```
goos: windows
goarch: amd64
pkg: linear_ac/lac
cpu: Intel(R) Core(TM) i7-10510U CPU @ 1.80GHz
BenchmarkNew-8                             46989            115594 ns/op             131 B/op          0 allocs/op
BenchmarkNewFrom-8                         50430            127946 ns/op             190 B/op          1 allocs/op
Benchmark_RawMalloc-8                    7590996             163.1 ns/op              88 B/op          5 allocs/op
Benchmark_LacMalloc-8                   15614894             81.11 ns/op               0 B/op          0 allocs/op
Benchmark_LacMallocMt-8                 12081428             90.54 ns/op               0 B/op          0 allocs/op
Benchmark_RawMallocLarge2-8                32990             40105 ns/op           27496 B/op       1656 allocs/op
Benchmark_LacMallocLarge2-8                79690             18879 ns/op            1028 B/op          0 allocs/op
Benchmark_LacMallocLarge2Mt-8              44228             26353 ns/op               0 B/op          0 allocs/op
```
- go test -bench . -tags='goexperiment.arenas' -benchmem
```
goos: windows
goarch: amd64
pkg: linear_ac/lac
cpu: Intel(R) Core(TM) i7-10510U CPU @ 1.80GHz
BenchmarkNew-8                             46694            114681 ns/op             132 B/op          0 allocs/op
BenchmarkNewFrom-8                         49684            122523 ns/op             192 B/op          1 allocs/op
Benchmark_RawMalloc-8                    6185844             183.2 ns/op              88 B/op          5 allocs/op
Benchmark_LacMalloc-8                   14673064             104.0 ns/op               0 B/op          0 allocs/op
Benchmark_LacMallocMt-8                 10891394             99.24 ns/op               0 B/op          0 allocs/op
Benchmark_RawMallocLarge2-8                25015             47155 ns/op           27496 B/op       1656 allocs/op
Benchmark_LacMallocLarge2-8                75883             18138 ns/op               6 B/op          0 allocs/op
Benchmark_LacMallocLarge2Mt-8              43879             26313 ns/op               0 B/op          0 allocs/op
Benchmark_RawMallocSmall-8               7762598             158.6 ns/op              88 B/op          5 allocs/op
Benchmark_LacMallocSmall-8              21674264             60.13 ns/op               0 B/op          0 allocs/op
Benchmark_ArenaMallocSmall-8             6486002             179.4 ns/op              87 B/op          0 allocs/op
Benchmark_RawMallocLarge-8                 24450             46036 ns/op           27496 B/op       1656 allocs/op
Benchmark_LacMallocLarge-8                 62058             27365 ns/op           27265 B/op          0 allocs/op
Benchmark_ArenaMallocLarge-8               10000            169460 ns/op           26978 B/op         45 allocs/op

```
