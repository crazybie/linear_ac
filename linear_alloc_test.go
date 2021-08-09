package auto_pb

import (
	"runtime"
	"testing"
	"unsafe"
)

// generated
type PbItem struct {
	Id     *int
	Price  *int
	Class  *int
	Name   *string
	Active *bool
}

// generated
type PbData struct {
	Age   *int
	Items []*PbItem
	InUse *PbItem
}

func Test_LinearAlloc(t *testing.T) {
	ac := NewLinearAllocator(100 * 1024)
	var d *PbData
	ac.New(&d)
	d.Age = ac.Int(11)

	var item *PbItem
	ac.New(&item)
	item.Id = ac.Int(2)
	item.Active = ac.Bool(true)
	item.Price = ac.Int(100)
	item.Class = ac.Int(3)
	item.Name = ac.String("name")

	ac.Append(&d.Items, item)

	if *d.Age != 11 {
		t.Errorf("age")
	}

	if len(d.Items) != 1 {
		t.Errorf("item")
	}
	if *d.Items[0].Id != 2 {
		t.Errorf("item.id")
	}

	ac.FreeAll()
}

func Test_Check(t *testing.T) {
	ac := NewLinearAllocator(100 * 1024)
	defer func() {
		if err := recover(); err == nil {
			t.Errorf("faile to check")
		}
	}()

	type D struct {
		v [4]*int
	}
	var d *D
	d = new(D)
	ac.New(&d)

	for i := 0; i < len(d.v); i++ {
		d.v[i] = new(int)
		*d.v[i] = i
	}
	ac.checkPointers()
}

func Test_WorkWithGc(t *testing.T) {
	type D struct {
		v [10]*int
	}

	ac := NewLinearAllocator(int(unsafe.Sizeof(D{})))
	defer ac.FreeAll()

	var d *D
	d = new(D)
	ac.New(&d)

	for i := 0; i < len(d.v); i++ {
		d.v[i] = ac.Int(i)
		//d.v[i] = new(int)
		*d.v[i] = i
		runtime.GC()
	}
	if ac.Miss != len(d.v) {
		t.Errorf("member int must use buildin allocator")
	}

	for i, p := range d.v {
		if *p != i {
			t.Errorf("int %v is gced", i)
		}
	}
}

func Benchmark_linearAlloc(t *testing.B) {
	t.ReportAllocs()
	ac := NewLinearAllocator(100 * 1024)
	defer ac.FreeAll()
	t.ResetTimer()

	for i := 0; i < t.N; i++ {
		var d *PbData
		ac.New(&d)
		d.Age = ac.Int(11)

		for j := 0; j < 100; j++ {
			var item *PbItem
			ac.New(&item)
			item.Id = ac.Int(2)
			item.Price = ac.Int(100)
			item.Class = ac.Int(3)
			item.Name = ac.String("name")

			ac.Append(&d.Items, item)
		}

		if *d.Age != 11 {
			t.Errorf("age")
		}
		if len(d.Items) != 100 {
			t.Errorf("item")
		}
		if *d.Items[0].Id != 2 {
			t.Errorf("item.id")
		}

		ac.FreeAll()
	}
}

func Benchmark_buildInAlloc(t *testing.B) {
	t.ReportAllocs()
	ac := NewLinearAllocator(0)
	t.ResetTimer()

	for i := 0; i < t.N; i++ {
		var d *PbData
		ac.New(&d)
		d.Age = ac.Int(11)

		for j := 0; j < 100; j++ {

			var item *PbItem
			ac.New(&item)
			item.Id = ac.Int(2)
			item.Price = ac.Int(100)
			item.Class = ac.Int(3)
			item.Name = ac.String("name")

			d.Items = append(d.Items, item)
		}

		if *d.Age != 11 {
			t.Errorf("age")
		}
		if len(d.Items) != 100 {
			t.Errorf("item")
		}
		if *d.Items[0].Id != 2 {
			t.Errorf("item.id")
		}
	}
}
