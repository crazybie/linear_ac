
# Linear Allocator

## Goal
Speed up the memory allocation and garbage collection performance.

## Compare with pool
1. more generel than pool.
Linear allocator can allocate different types of objects.
3. reduce the gc object scanning overhead.
The allocator is just a few byte array.
5. much simper and faster on recliming memores.
No need to mannually release each object allocated from the linear allocator, just reset the allocation cursor and all is done.

## Possible Usage
1. global memory never need to be released. (configs, global systems)
2. temporary objects with deterministic lifetime. (buffers send to network)

## Note
1. don't assign memories allocated from build-in allocator to linear allocated objects.
2. don't store the pointers of linear allocated objects after allocator released.


## TODO:
1. SliceAppend support value type as elem

## Usage:

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

```

## Benchmark
Results from benchmark tests:
``` 
Benchmark_linearAlloc
Benchmark_linearAlloc-8    	    3890	    277651 ns/op	      27 B/op	       0 allocs/op
Benchmark_buildInAlloc
Benchmark_buildInAlloc-8   	    4744	    254372 ns/op	  112440 B/op	    6013 allocs/op
```
