
# Linear Allocator for Golang

## Goal
Speed up the memory allocation and improve the GC performance, especially for dynamic-memory-heavy applications.

## Potential UseCases
1. Large amount of memory never needs to be released. (global configs, readonly assets like navmesh)
2. Massive temporary objects with deterministic lifetime. (protobuf objects send to network)

## Advantages over pool
Linear allocator:

1. Can greatly reduce the object scanning pressure of GC. Linear allocator is just a few byte arrays internally, but pool is normal container always need to be scanned fully.
2. More general. Linear allocator can allocate various type of objects.
3. Much simpler and faster on reclaiming memories. No need to manually release every object back but just reset the allocation cursor.
4. Much cheaper. Linear allocators share memory chunks via chunk pool. 
5. Memory efficient. Memories are more compact, cpu cache friendly. 

## Limitations
1. Don't store the pointers of build-in allocated objects into linear allocated objects. (There's a debug flag for checking external pointers)
2. Don't store and use the pointers of linear allocated objects after the allocator is reset or released.
3. Not support concurrency. 


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
	
	ac := Get()
	defer func(){
		ac.Release()
	}()
	
	var d *PbData
	ac.New(&d)
	d.Age = ac.Int(11)

	n := 3
	for i := 0; i < n; i++ {
		var item *PbItem
		ac.New(&item)
		item.Id = ac.Int(i + 1)
		item.Active = ac.Bool(true)
		item.Price = ac.Int(100 + i)
		item.Class = ac.Int(3 + i)
		item.Name = ac.String("name")

		ac.SliceAppend(&d.Items, item)
	}

	ac.Reset()
}
```

## Benchmark
Results from benchmark tests:
![bench](./bench.png)
``` 
cpu: Intel(R) Core(TM) i7-10510U CPU @ 1.80GHz
Benchmark_linearAllocNoGC
Benchmark_linearAllocNoGC-8       429055              2806 ns/op               8 B/op          0 allocs/op
Benchmark_buildInAllocNoGc
Benchmark_buildInAllocNoGc-8      333940              3904 ns/op            2244 B/op        148 allocs/op
Benchmark_linearAllocGc
Benchmark_linearAllocGc-8         422084              2844 ns/op               8 B/op          0 allocs/op
Benchmark_buildInAllocGc
Benchmark_buildInAllocGc-8        352863              5918 ns/op            2244 B/op        148 allocs/op
Benchmark_linearAllocGc2
Benchmark_linearAllocGc2-8        277922              4003 ns/op               8 B/op          0 allocs/op
Benchmark_buildInAllocGc2
Benchmark_buildInAllocGc2-8       265830             20012 ns/op            2244 B/op        148 allocs/op
Benchmark_linearAllocGc3
Benchmark_linearAllocGc3-8        220708              5783 ns/op               9 B/op          0 allocs/op
Benchmark_buildInAllocGc3
Benchmark_buildInAllocGc3-8       145726             94274 ns/op            2244 B/op        148 allocs/op

```
