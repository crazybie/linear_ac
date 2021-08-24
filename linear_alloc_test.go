package linear_ac

import (
	"fmt"
	"runtime"
	"testing"
)

type EnumA int32

const (
	EnumVal1 EnumA = 1
	EnumVal2 EnumA = 2
)

type PbItem struct {
	Id      *int
	Price   *int
	Class   *int
	Name    *string
	Active  *bool
	EnumVal *EnumA
}

type PbData struct {
	Age   *int
	Items []*PbItem
	InUse *PbItem
}

func Test_LinearAlloc(t *testing.T) {
	ac := NewLinearAc()
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
	DbgCheckPointers = 1
	ac := NewLinearAc()
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
	ac.CheckPointers()
}

func Test_WorkWithGc(t *testing.T) {
	type D struct {
		v [10]*int
	}

	ac := NewLinearAc()

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
	ac := NewLinearAc()

	type D struct {
		s [5]*string
	}
	var d *D
	ac.New(&d)
	for i := range d.s {
		d.s[i] = ac.String(fmt.Sprintf("str%v", i))
		runtime.GC()
	}
	for i, p := range d.s {
		if *p != fmt.Sprintf("str%v", i) {
			t.Errorf("elem %v is gced", i)
		}
	}
}

func TestLinearAllocator_NewMap(t *testing.T) {
	ac := NewLinearAc()

	type D struct {
		m map[int]*int
	}
	data := [10]*D{}
	for i := 0; i < len(data); i++ {
		var d *D
		ac.New(&d)
		ac.NewMap(&d.m)
		d.m[1] = ac.Int(i)
		data[i] = d
		runtime.GC()
	}
	for i, d := range data {
		if *d.m[1] != i {
			t.Fail()
		}
	}
}

func TestLinearAllocator_ExternalMap(t *testing.T) {
	DbgCheckPointers = 1
	ac := NewLinearAc()
	defer func() {
		if err := recover(); err == nil {
			t.Errorf("faile to check")
		}
	}()

	type D struct {
		m map[int]*int
	}
	var d *D
	ac.New(&d)
	d.m = make(map[int]*int)
	ac.CheckPointers()
}

func TestLinearAllocator_NewSlice(t *testing.T) {
	ac := NewLinearAc()
	s := make([]*int, 0)
	ac.SliceAppend(&s, ac.Int(2))
	if len(s) != 1 && *s[0] != 2 {
		t.Fail()
	}

	ac.NewSlice(&s, 0, 32)
	ac.SliceAppend(&s, ac.Int(1))
	if cap(s) != 32 || *s[0] != 1 {
		t.Fail()
	}
}

func TestLinearAllocator_New2(b *testing.T) {
	ac := NewLinearAc()
	for i := 0; i < 3; i++ {
		d := ac.New2(&PbItem{
			Id:    ac.Int(1 + i),
			Class: ac.Int(2 + i),
			Price: ac.Int(3 + i),
			Name:  ac.String("test"),
		}).(*PbItem)

		if *d.Id != 1+i {
			b.Fail()
		}
		if *d.Class != 2+i {
			b.Fail()
		}
		if *d.Price != 3+i {
			b.Fail()
		}
		if *d.Name != "test" {
			b.Fail()
		}
	}
}

func TestAllocator_EnumInt32(t *testing.T) {
	ac := NewLinearAc()
	e := EnumVal1
	v := ac.Enum(e).(*EnumA)
	if *v != e {
		t.Fail()
	}
}

func TestBuildInAllocator_All(t *testing.T) {
	ac := BuildInAc
	var item *PbItem
	ac.New(&item)
	item.Id = ac.Int(11)
	if *item.Id != 11 {
		t.Fail()
	}
	id2 := 22
	item = ac.New2(&PbItem{Id: &id2}).(*PbItem)
	if *item.Id != 22 {
		t.Fail()
	}
	var s []*PbItem
	ac.NewSlice(&s, 0, 3)
	if cap(s) != 3 || len(s) != 0 {
		t.Fail()
	}
	ac.SliceAppend(&s, item)
	if len(s) != 1 || *s[0].Id != 22 {
		t.Fail()
	}
	var m map[int]string
	ac.NewMap(&m)
	m[1] = "test"
	if m[1] != "test" {
		t.Fail()
	}
}

func Benchmark_linearAlloc(t *testing.B) {
	t.ReportAllocs()
	DbgCheckPointers = 0
	var ac = NewLinearAc()
	defer func() {
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
			if j%2 == 0 {
				ac.New(&item)
				item.Id = ac.Int(2 + j)
				item.Price = ac.Int(100 + j)
				item.Class = ac.Int(3 + j)
				item.Name = ac.String("name")
				item.EnumVal = ac.Enum(EnumVal2).(*EnumA)
			} else {
				item = ac.New2(&PbItem{
					Id:      ac.Int(2 + j),
					Price:   ac.Int(100 + j),
					Class:   ac.Int(3 + j),
					Name:    ac.String("name"),
					EnumVal: ac.Enum(EnumVal2).(*EnumA),
				}).(*PbItem)
			}

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

		if i%200 == 0 {
			runtime.GC()
		}
		ac.Reset()
	}
}

func Benchmark_buildInAlloc(t *testing.B) {
	t.ReportAllocs()

	newInt := func(v int) *int { return &v }
	newStr := func(v string) *string { return &v }
	newBool := func(v bool) *bool { return &v }
	preventFromGc := make([]*PbData, 0, t.N)
	enum := func(v EnumA) *EnumA { return &v }

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
			item.EnumVal = enum(EnumVal2)

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
		if i%200 == 0 {
			runtime.GC()
		}
		preventFromGc = append(preventFromGc, d)
	}
	if len(preventFromGc) != t.N {
		t.Fail()
	}
}
