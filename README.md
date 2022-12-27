
# Linear Allocator for Golang

## Goal
Speed up the memory allocation and improve the GC performance, especially for dynamic-memory-heavy applications.

NOTE: current version need go1.18+.

## Potential UseCases
1. A large amount of memory never needs to be released. (global configs, read-only assets like navmesh)
2. Massive temporary objects with deterministic lifetime. (protobuf objects send to network)


# Pros than v1.20 Arena
1. much faster on allocating(see benchmark results below), gc marking and sweeping.
2. support concurrency.
3. support debugging mode.


## Highlights
Linear allocator:

1. Can greatly reduce the object scanning pressure of GC. Linear allocator is just a few byte arrays internally, but pool is normal container always need to be scanned fully.
2. More general. Linear allocator can allocate various types of objects.
3. Much simpler and faster on reclaiming memories. No need to manually release every object back but just reset the allocation cursor.
4. Much cheaper. Linear allocators reuse memory chunks among each other via chunk pool. 
5. Memory efficient. Memories are more compact, CPU cache-friendly.
6. Allows build-in allocated objects to be attached to the lac allocated objects. 
7. Support concurrency.


## Limitations
1. Don't store the pointers of build-in allocated objects into linear allocated objects directly. (There's a debug mode for checking external pointers)
2. Don't store and use the pointers of linear allocated objects after the allocator is reset or released. (In debug mode, the allocator traverses the objects and obfuscate the pointers to make any attempting usage panic)


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
- Allocation performance compare with arena allocator of v1.20.\
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

