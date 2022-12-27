
# LAC - Linear Allocator for Golang

## Goal
Speed up the memory allocation and improve the GC performance, especially for dynamic-memory-heavy applications.

NOTE: current version need go1.18+.

## Potential UseCases
1. A large amount of memory never needs to be released. (global configs, read-only assets like navmesh)
2. Massive temporary objects with deterministic lifetime. (protobuf objects send to network)


# Pros than v1.20 Arena
1. much faster on allocating(see benchmark results below), gc marking and sweeping.
2. support concurrency.
3. slice append can utilize the linear allocator as well. 
4. support debugging mode.


## Highlights
Linear allocator:

1. Mush fast on memory allocating. An allocation is just a pointer adjustment internally.
2. Can greatly reduce the object scanning pressure of GC. LAC is just a few byte arrays internally, but pool is normal container always need to be scanned fully.
3. More general. LAC can allocate various types of objects.
4. Much simpler and faster on reclaiming memories. No need to manually release every object back but just reset the allocation cursor.
5. Much cheaper. LACs reuse memory chunks among each other via chunk pool. 
6. Memory efficient. Memories are more compact, CPU cache-friendly.
7. Allows build-in allocated objects to be attached to the LAC allocated objects. 
8. Support concurrency.


## Limitations
1. Don't store the pointers of build-in allocated objects into LAC allocated objects directly. (There's a debug mode for checking external pointers)
2. Don't store and use the pointers of LAC allocated objects after the allocator is reset or released. (In debug mode, the allocator traverses the objects and obfuscate the pointers to make any attempting usage panic)
3. map memory can't use LAC and fallback to build-in one.


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

		ac.SliceAppend(&d.Items, item)
	}
}
```

## Benchmark
Results from benchmark tests:

- GC overhead\
![bench](./bench.png)
- Allocation performance compare with arena allocator of v1.20.
```
cpu: Intel(R) Core(TM) i7-10510U CPU @ 1.80GHz
Benchmark_LacMallocLarge
Benchmark_LacMallocLarge-8         25328             42923 ns/op
Benchmark_ArenaMallocLarge
Benchmark_ArenaMallocLarge-8       19467             60022 ns/op
Benchmark_LacMallocSmall
Benchmark_LacMallocSmall-8      11035862               109.7 ns/op
Benchmark_ArenaMallocSmall
Benchmark_ArenaMallocSmall-8     7336266               165.0 ns/op
```
- Latency under heavy allocation case. 
``` 
Benchmark_LinearAc
>> Latency: max=944ms, avg=6ms.
Benchmark_LinearAc-8                   1        9589733200 ns/op
Benchmark_buildInAc
>> Latency: max=3535ms, avg=7ms.
Benchmark_buildInAc-8                  1        7651476400 ns/op

```

