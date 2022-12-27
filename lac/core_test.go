/*
 * Linear Allocator
 *
 * Improve the memory allocation and garbage collection performance.
 *
 * Copyright (C) 2020-2021 crazybie@github.com.
 * https://github.com/crazybie/linear_ac
 */

package lac

import (
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"testing"
	"time"
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

func Test_GoRoutineId(t *testing.T) {
	id := goRoutineId()
	if id != goRoutineIdSlow() {
		t.Fail()
	}
}

func Test_LinearAlloc(t *testing.T) {
	ac := BindNew()
	d := New[PbData](ac)
	d.Age = ac.Int(11)

	n := 3
	for i := 0; i < n; i++ {
		item := New[PbItem](ac)
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

	if len(d.Items) != int(n) {
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
	ac.reset()
	ac.Release()
}

func Test_String(t *testing.T) {
	ac := BindNew()

	type D struct {
		s [5]*string
	}
	d := New[D](ac)
	for i := range d.s {
		d.s[i] = ac.String(fmt.Sprintf("str%v", i))
		runtime.GC()
	}
	for i, p := range d.s {
		if *p != fmt.Sprintf("str%v", i) {
			t.Errorf("elem %v is gced", i)
		}
	}
	ac.Release()
}

func TestLinearAllocator_NewMap(t *testing.T) {
	ac := BindNew()

	type D struct {
		m map[int]*int
	}
	data := [10]*D{}
	for i := 0; i < len(data); i++ {
		d := New[D](ac)
		d.m = NewMap[int, *int](ac)
		d.m[1] = ac.Int(i)
		data[i] = d
		runtime.GC()
	}
	for i, d := range data {
		if *d.m[1] != i {
			t.Fail()
		}
	}
	ac.Release()
}

func TestLinearAllocator_NewSlice(t *testing.T) {
	DbgMode = true
	ac := BindNew()
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

	intSlice := []int{}
	ac.SliceAppend(&intSlice, 11)
	if len(intSlice) != 1 || intSlice[0] != 11 {
		t.Fail()
	}

	byteSlice := []byte{}
	ac.SliceAppend(&byteSlice, byte(11))
	if len(byteSlice) != 1 || byteSlice[0] != 11 {
		t.Fail()
	}

	type Data struct {
		d [2]uint64
	}
	structSlice := []Data{}
	d1 := uint64(0xcdcdefefcdcdefdc)
	d2 := uint64(0xcfcdefefcdcfffde)
	ac.SliceAppend(&structSlice, Data{d: [2]uint64{d1, d2}})
	if len(structSlice) != 1 || structSlice[0].d[0] != d1 || structSlice[0].d[1] != d2 {
		t.Fail()
	}

	f := func() []int {
		var r []int = NewSlice[int](ac, 0, 1)
		ac.SliceAppend(&r, 1)
		return r
	}
	r := f()
	if len(r) != 1 {
		t.Errorf("return slice")
	}

	{
		var s []*PbItem
		ac.SliceAppend(&s, nil)
		if len(s) != 1 || s[0] != nil {
			t.Errorf("nil")
		}
	}

	{
		s := []int{2, 3, 4}
		r := ac.CopySlice(s).([]int)
		s[0] = 1
		s[1] = 1
		s[2] = 1
		if len(r) != len(s) {
			t.Errorf("copy")
		}
		for i := range s {
			if s[i] == r[i] {
				t.Errorf("copy elem")
			}
		}
	}

	ac.Release()
}

func TestLinearAllocator_NewCopy(b *testing.T) {
	ac := BindNew()
	for i := 0; i < 3; i++ {
		d := NewCopy(ac, &PbItem{
			Id:    ac.Int(1 + i),
			Class: ac.Int(2 + i),
			Price: ac.Int(3 + i),
			Name:  ac.String("test"),
		})

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
	ac.Release()
}

func TestAllocator_Enum(t *testing.T) {
	ac := BindNew()
	e := EnumVal2
	v := NewEnum(ac, e)
	if *v != e {
		t.Fail()
	}
	ac.Release()
}

func TestBuildInAllocator_All(t *testing.T) {
	ac := BuildInAc
	item := New[PbItem](ac)
	item.Id = ac.Int(11)
	if *item.Id != 11 {
		t.Fail()
	}
	id2 := 22
	item = NewCopy(ac, &PbItem{Id: &id2})
	if *item.Id != 22 {
		t.Fail()
	}
	s := NewSlice[*PbItem](ac, 0, 3)
	if cap(s) != 3 || len(s) != 0 {
		t.Fail()
	}
	ac.SliceAppend(&s, item)
	if len(s) != 1 || *s[0].Id != 22 {
		t.Fail()
	}
	m := NewMap[int, string](ac)
	m[1] = "test"
	if m[1] != "test" {
		t.Fail()
	}
	e := EnumVal1
	v := NewEnum(ac, e)
	if *v != e {
		t.Fail()
	}
	ac.Release()
}

func TestBindAc(t *testing.T) {
	useAc := func() *Allocator {
		return Get()
	}

	wg := sync.WaitGroup{}
	for i := 0; i < 1000; i++ {
		go func() {
			wg.Add(1)

			ac := BindNew()
			defer ac.Release()

			time.Sleep(time.Duration(rand.Float32()*1000) * time.Millisecond)

			if useAc() != ac {
				t.Fail()
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func TestLinearAllocator_NewExternalPtr(b *testing.T) {
	ac := BindNew()
	type D struct {
		d [10]*int
	}
	d := New[D](ac)
	for i := 0; i < len(d.d); i++ {
		d.d[i] = AttachExternalPtr(ac, new(int))
		//d.d[i] = new(int)
		*d.d[i] = i
		runtime.GC()
	}
	for i := 0; i < len(d.d); i++ {
		if *d.d[i] != i {
			b.Errorf("should not be gced.")
		}
	}
	ac.Release()
}

func NoMalloc(f func()) {
	var s, e runtime.MemStats
	runtime.ReadMemStats(&s)
	f()
	runtime.ReadMemStats(&e)
	if n := e.Mallocs - s.Mallocs; n > 0 {
		panic(fmt.Errorf("has %v malloc", n))
	}
}

func TestLinearAllocator_NewCopyNoAlloc(b *testing.T) {
	chunkPool.reserve(1)
	ac := BindNew()
	defer ac.Release()

	var r *PbItem
	NoMalloc(func() {
		r = NewCopy(ac, &PbItem{})
		//r = new(PbItem)
	})
	runtime.KeepAlive(r)
}

func TestLinearAllocator_Alignment(t *testing.T) {
	ac := Get()
	defer ac.Release()

	for i := 0; i < 1024; i++ {
		p := ac.alloc(i, false)
		if (uintptr(p) & uintptr(PtrSize-1)) != 0 {
			t.Fail()
		}
	}
}
