package linear_ac

import (
	"fmt"
	"runtime"
	"testing"
)

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

func Test_LinearAlloc(t *testing.T) {
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

	if *d.Age != 11 {
		t.Errorf("age")
	}

	if len(d.Items) != n {
		t.Errorf("item")
	}
	for i := 0; i < n; i++ {
		if *d.Items[i].Id != i+1 {
			t.Errorf("item.id")
		}
		if *d.Items[i].Price != i+100 {
			t.Errorf("item.price")
		}
		if *d.Items[i].Class != i+3 {
			t.Errorf("item.class")
		}
	}
	ac.Reset()
}

func Test_Check(t *testing.T) {
	ac := NewLinearAllocator()
	defer func() {
		if err := recover(); err == nil {
			t.Errorf("faile to check")
		}
	}()

	type D struct {
		v [4]*int
	}
	var d *D
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

	ac := NewLinearAllocator()
	defer ac.Reset()

	var d *D
	ac.New(&d)

	for i := 0; i < len(d.v); i++ {
		d.v[i] = ac.Int(i)
		//d.v[i] = new(int)
		*d.v[i] = i
		runtime.GC()
	}

	for i, p := range d.v {
		if *p != i {
			t.Errorf("int %v is gced", i)
		}
	}
}

func Test_String(t *testing.T) {
	ac := NewLinearAllocator()
	defer ac.Reset()

	type D struct {
		s [5]*string
	}
	var d *D
	ac.New(&d)
	for i, _ := range d.s {
		d.s[i] = ac.String(fmt.Sprintf("str%v", i))
		runtime.GC()
	}
	for i, p := range d.s {
		if *p != fmt.Sprintf("str%v", i) {
			t.Errorf("elem %v is gced", i)
		}
	}
}

func Benchmark_linearAlloc(t *testing.B) {
	t.ReportAllocs()
	DbgCheckPointers = 0
	ac := NewLinearAllocator()
	defer func() {
		ac.Reset()
		DbgCheckPointers = 1
	}()
	t.StartTimer()

	for i := 0; i < t.N; i++ {
		var d *PbData
		ac.New(&d)
		d.Age = ac.Int(11)

		n := 1000
		for j := 0; j < n; j++ {
			var item *PbItem
			ac.New(&item)
			item.Id = ac.Int(2 + j)
			item.Price = ac.Int(100 + j)
			item.Class = ac.Int(3 + j)
			item.Name = ac.String("name")

			ac.SliceAppend(&d.Items, item)
		}

		if *d.Age != 11 {
			t.Errorf("age")
		}
		if len(d.Items) != n {
			t.Errorf("item")
		}
		for j := 0; j < n; j++ {
			if *d.Items[j].Id != 2+j {
				t.Errorf("item.id")
			}
		}

		ac.Reset()
	}
	t.StopTimer()
}

func Benchmark_buildInAlloc(t *testing.B) {
	t.ReportAllocs()

	newInt := func(v int) *int { return &v }
	newStr := func(v string) *string { return &v }
	newBool := func(v bool) *bool { return &v }
	preventFromGc := make([]*PbData, 0, t.N)

	t.StartTimer()
	for i := 0; i < t.N; i++ {
		var d *PbData = new(PbData)
		d.Age = newInt(11)

		n := 1000
		for j := 0; j < n; j++ {

			var item *PbItem = new(PbItem)
			item.Id = newInt(2 + j)
			item.Price = newInt(100 + j)
			item.Class = newInt(3 + j)
			item.Name = newStr("name")
			item.Active = newBool(true)

			d.Items = append(d.Items, item)
		}

		if *d.Age != 11 {
			t.Errorf("age")
		}
		if len(d.Items) != n {
			t.Errorf("item")
		}
		for j := 0; j < n; j++ {
			if *d.Items[j].Id != 2+j {
				t.Errorf("item.id")
			}
		}
		preventFromGc = append(preventFromGc, d)
	}
	t.StopTimer()
	if len(preventFromGc) != t.N {
		t.Fail()
	}
}
