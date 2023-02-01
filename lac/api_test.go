/*
 * Linear Allocator
 *
 * Improve the memory allocation and garbage collection performance.
 *
 * Copyright (C) 2020-2023 crazybie@github.com.
 * https://github.com/crazybie/linear_ac
 */

package lac

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
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

func Test_Smoke(t *testing.T) {
	ac := Get()
	defer ac.Release()

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
		d.Items = Append(ac, d.Items, item)
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
}

func Test_Alignment(t *testing.T) {
	ac := Get()
	defer ac.Release()

	for i := 0; i < 1024; i++ {
		p := ac.alloc(i, false)
		if (uintptr(p) & uintptr(PtrSize-1)) != 0 {
			t.Fail()
		}
	}
}

func Test_String(t *testing.T) {
	ac := Get()
	defer ac.Release()

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
}

func Test_NewMap(t *testing.T) {
	ac := Get()
	defer ac.Release()

	type D struct {
		m map[int]*int
	}
	data := [10]*D{}
	for i := 0; i < len(data); i++ {
		d := New[D](ac)
		d.m = NewMap[int, *int](ac, 0)
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

func Test_NewSlice(t *testing.T) {
	EnableDebugMode(true)
	ac := Get()
	defer ac.Release()

	s := make([]*int, 0)
	s = Append(ac, s, ac.Int(2))
	if len(s) != 1 && *s[0] != 2 {
		t.Fail()
	}

	s = NewSlice[*int](ac, 0, 32)
	s = Append(ac, s, ac.Int(1))
	if cap(s) != 32 || *s[0] != 1 {
		t.Fail()
	}

	intSlice := []int{}
	intSlice = Append(ac, intSlice, 11)
	if len(intSlice) != 1 || intSlice[0] != 11 {
		t.Fail()
	}

	byteSlice := []byte{}
	byteSlice = Append(ac, byteSlice, byte(11))
	if len(byteSlice) != 1 || byteSlice[0] != 11 {
		t.Fail()
	}

	type Data struct {
		d [2]uint64
	}
	structSlice := []Data{}
	d1 := uint64(0xcdcdefefcdcdefdc)
	d2 := uint64(0xcfcdefefcdcfffde)
	structSlice = Append(ac, structSlice, Data{d: [2]uint64{d1, d2}})
	if len(structSlice) != 1 || structSlice[0].d[0] != d1 || structSlice[0].d[1] != d2 {
		t.Fail()
	}

	f := func() []int {
		var r []int = NewSlice[int](ac, 0, 1)
		r = Append(ac, r, 1)
		return r
	}
	r := f()
	if len(r) != 1 {
		t.Errorf("return slice")
	}

	{
		var s []*PbItem
		s = Append(ac, s, nil)
		if len(s) != 1 || s[0] != nil {
			t.Errorf("nil")
		}
	}
}

func Test_NewFromRaw(b *testing.T) {
	var ac *Allocator

	for i := 0; i < 3; i++ {
		d := NewFrom(ac, &PbItem{
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
}

func Test_NewFrom(b *testing.T) {
	ac := Get()
	defer ac.Release()

	for i := 0; i < 3; i++ {
		d := NewFrom(ac, &PbItem{
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
}

func Test_BuildInAllocator(t *testing.T) {
	var ac *Allocator
	defer ac.Release()

	item := New[PbItem](ac)
	item.Id = ac.Int(11)
	if *item.Id != 11 {
		t.Fail()
	}
	id2 := 22
	item = NewFrom(ac, &PbItem{Id: &id2})
	if *item.Id != 22 {
		t.Fail()
	}
	s := NewSlice[*PbItem](ac, 0, 3)
	if cap(s) != 3 || len(s) != 0 {
		t.Fail()
	}
	s = Append(ac, s, item)
	if len(s) != 1 || *s[0].Id != 22 {
		t.Fail()
	}
	m := NewMap[int, string](ac, 0)
	m[1] = "test"
	if m[1] != "test" {
		t.Fail()
	}
	e := EnumVal1
	v := NewEnum(ac, e)
	if *v != e {
		t.Fail()
	}
}

func Test_Enum(t *testing.T) {
	ac := Get()
	defer ac.Release()

	e := EnumVal2
	v := NewEnum(ac, e)
	if *v != e {
		t.Fail()
	}
}

func Test_AttachExternal(b *testing.T) {
	ac := Get()
	defer ac.Release()

	type D struct {
		d [10]*int
	}
	d := New[D](ac)
	for i := 0; i < len(d.d); i++ {
		d.d[i] = Attach(ac, new(int))
		//d.d[i] = new(int)
		*d.d[i] = i
		runtime.GC()
	}

	for i := 0; i < len(d.d); i++ {
		if *d.d[i] != i {
			b.Errorf("should not be gced.")
		}
	}
}

func Test_Append(t *testing.T) {
	ac := Get()
	defer ac.Release()

	m := map[int][]int{}

	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			m[i] = Append(ac, m[i], j)
		}
	}

	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			if m[i][j] != j {
				t.Fail()
			}
		}
	}
}

func Test_SliceAppendStructValue(t *testing.T) {
	ac := Get()
	defer ac.Release()

	type S struct {
		a int
		b float32
		c string
	}

	var s []S
	s = Append(ac, s, S{1, 2, "3"})
	if s[0].a != 1 || s[0].b != 2 || s[0].c != "3" {
		t.Fail()
	}
}

func Test_NilAc(t *testing.T) {
	var ac *Allocator

	type S struct {
		v int
	}
	o := New[S](ac)
	if o == nil {
		t.Fail()
	}

	f := NewFrom(ac, &S{})
	if f == nil {
		t.Fail()
	}

	m := NewMap[int, int](ac, 1)
	if m == nil {
		t.Fail()
	}

	s := NewSlice[byte](ac, 10, 10)
	if cap(s) != 10 || len(s) != 10 {
		t.Fail()
	}

	e := NewEnum(ac, EnumVal2)
	if *e != EnumVal2 {
		t.Fail()
	}

	i := ac.Int(1)
	if *i != 1 {
		t.Fail()
	}

	ss := ac.String("ss")
	if *ss != "ss" {
		t.Fail()
	}

	ac.IncRef()
	ac.DecRef()
	ac.Release()
}

func Test_SliceWrongCap(t *testing.T) {
	defer func() {
		if err := recover(); err == nil {
			panic(fmt.Errorf("should panic: out of range"))
		}
	}()
	ac := Get()
	defer ac.Release()
	NewSlice[byte](ac, 10, 0)
}

// NOTE: run with "-race".
func TestSharedAc_NoRace(t *testing.T) {
	ac := Get()
	wg := sync.WaitGroup{}
	wg.Add(100)

	for i := 0; i < 100; i++ {
		ac.IncRef()
		go func() {

			var item *PbItem
			for j := 0; j < 1000; j++ {
				item = New[PbItem](ac)
				item.Class = Attach(ac, new(int))
				*item.Class = j
				item.Id = ac.Int(j)
			}
			runtime.KeepAlive(item)

			ac.DecRef()
			wg.Done()
		}()
	}

	ac.DecRef()
	wg.Wait()
	if n := atomic.LoadInt32(&ac.refCnt); n != 1 {
		t.Errorf("ref cnt:%v", n)
	}
}
