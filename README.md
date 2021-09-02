
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
3. Much simpler and faster on reclaiming memories. No need to manually release every object back, just reset the allocation cursor.
4. Cheaper. Linear allocator do allocations on-demand like pool, but can be thrown away like temporary object if you don't want to reuse it. 
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
``` 
goos: linux
goarch: amd64
pkg: oops/common/linear_alloc
Benchmark_linearAllocNoGC
Benchmark_linearAllocNoGC-6    	  366729	      3117 ns/op	       8 B/op	       0 allocs/op
Benchmark_buildInAllocNoGc
Benchmark_buildInAllocNoGc-6   	  306507	      4176 ns/op	    2244 B/op	     148 allocs/op
Benchmark_linearAllocGc
Benchmark_linearAllocGc-6      	  379468	      3138 ns/op	       8 B/op	       0 allocs/op
Benchmark_buildInAllocGc
Benchmark_buildInAllocGc-6     	  305342	      4861 ns/op	    2244 B/op	     148 allocs/op
Benchmark_linearAllocGc2
Benchmark_linearAllocGc2-6     	  381771	      3346 ns/op	       8 B/op	       0 allocs/op
Benchmark_buildInAllocGc2
Benchmark_buildInAllocGc2-6    	  329953	     15584 ns/op	    2244 B/op	     148 allocs/op
Benchmark_linearAllocGc3
Benchmark_linearAllocGc3-6     	  358688	      7520 ns/op	       8 B/op	       0 allocs/op
Benchmark_buildInAllocGc3
Benchmark_buildInAllocGc3-6    	  157050	     55906 ns/op	    2244 B/op	     148 allocs/op

```
