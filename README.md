
 # Linear Allocator

 ## Goal
 Speed up the memory allocation and garbage collection performance.

 ## Possible Usage
 1. global memory never need to be released. (configs, global systems)
 2. temporary objects with deterministic lifetime. (buffers send to network)

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
