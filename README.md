
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
Benchmark_linearAlloc
Benchmark_linearAlloc-6    	   27282	     45661 ns/op	     229 B/op	       7 allocs/op
Benchmark_buildInAlloc
Benchmark_buildInAlloc-6   	   10000	    302245 ns/op	   23352 B/op	    1411 allocs/op
```
