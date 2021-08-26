
# Linear Allocator for Golang

## Goal
Speed up the memory allocation and improve the GC performance, espacially for dynamic memory heavy applications.

## Possible Usecases
1. Large amount of global memory never needs to be released. (configs, readonly assets like navmesh)
2. Massive temporary objects with deterministic lifetime. (protobuf objects send to network)

## Compare with pool
1. More general. The linear allocator can allocate various type of objects.
2. Greatly reduce the object scanning presure of GC. Linear allocator is just a few byte arrays internally, but pool is normal container allways need to be scanned. 
3. Much simpler and faster on reclaiming memories. No need to manually release every object back, just reset the allocation cursor.

## Limitations
1. Don't store the pointers of build-in allocated objects into linear allocated objects. (There's a debug flag for checking external pointers)
2. Don't store and use the pointers of linear allocated objects after the allocator is reset or released.



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


// Usage

ac := NewLinearAllocator()
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
```

## Benchmark
Results from benchmark tests:
``` 
cpu: Intel(R) Core(TM) i7-10510U CPU @ 1.80GHz
Benchmark_linearAlloc
Benchmark_linearAlloc-8             2751            377632 ns/op              44 B/op          0 allocs/op
Benchmark_buildInAlloc
Benchmark_buildInAlloc-8            3436           1523688 ns/op          112440 B/op       7013 allocs/op
```
