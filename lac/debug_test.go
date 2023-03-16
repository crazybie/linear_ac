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
	"runtime"
	"sync"
	"testing"
)

var acPool = NewAllocatorPool("test", nil, 10000, 64*1024, 32*1000, 64*1000)
var acPoolMu sync.RWMutex

func Test_CheckArray(t *testing.T) {
	acPool.EnableDebugMode(true)
	ac := acPool.Get()

	defer func() {
		if err := recover(); err == nil {
			t.Errorf("faile to check")
		}
	}()

	type D struct {
		v [4]*int
	}

	d := New[D](ac)
	for i := 0; i < len(d.v); i++ {
		d.v[i] = new(int)
		*d.v[i] = i
	}
	ac.Release()
}

func Test_CheckInternalSlice(t *testing.T) {
	acPool.EnableDebugMode(true)
	ac := acPool.Get()
	defer ac.Release()

	type D struct {
		v []int
	}
	d := New[D](ac)
	d.v = NewSlice[int](ac, 1, 1)
}

func Test_CheckExternalSlice(t *testing.T) {
	acPool.EnableDebugMode(true)
	ac := acPool.Get()

	defer func() {
		if err := recover(); err == nil {
			t.Errorf("faile to check")
		}
	}()

	type D struct {
		v []*int
	}
	d := New[D](ac)

	d.v = make([]*int, 3)
	for i := 0; i < len(d.v); i++ {
		d.v[i] = new(int)
		*d.v[i] = i
	}

	ac.Release()
}

func Test_CheckKnownExternalSlice(t *testing.T) {
	acPool.EnableDebugMode(true)
	ac := acPool.Get()
	defer ac.Release()

	defer func() {
		if err := recover(); err != nil {
			t.Errorf("faile to check")
		}
	}()

	type D struct {
		v []*int
	}
	d := New[D](ac)

	d.v = Attach(ac, make([]*int, 3))
	for i := 0; i < len(d.v); i++ {
		d.v[i] = Attach(ac, new(int))
		*d.v[i] = i
	}
}

func TestUseAfterFree_Pointer(t *testing.T) {
	acPool.EnableDebugMode(true)
	ac := acPool.Get()

	defer func() {
		acPool.EnableDebugMode(false)
		if err := recover(); err == nil {
			t.Errorf("failed to check")
		}
	}()

	d := New[PbData](ac)
	d.Age = ac.Int(11)
	ac.Release()
	if *d.Age == 11 {
		t.Errorf("not panic")
	}
}

func TestUseAfterFree_StructPointer(t *testing.T) {
	acPool.EnableDebugMode(true)
	ac := acPool.Get()
	defer func() {
		acPool.EnableDebugMode(false)
		if err := recover(); err == nil {
			t.Errorf("failed to check")
		}
	}()

	d := New[PbData](ac)
	d.InUse = New[PbItem](ac)

	ac.Release()
	c := *d.InUse
	t.Errorf("should panic")
	_ = c
}

func TestUseAfterFree_Slice(t *testing.T) {
	acPool.EnableDebugMode(true)
	ac := acPool.Get()

	defer func() {
		acPool.EnableDebugMode(false)
		if err := recover(); err == nil {
			t.Errorf("failed to check")
		}
	}()

	d := New[PbData](ac)
	d.Items = NewSlice[*PbItem](ac, 1, 1)
	ac.Release()

	if cap(d.Items) == 1 {
		t.Errorf("not panic")
	}
	d.Items[0] = new(PbItem)
}

func Test_WorkWithGc(t *testing.T) {
	type D struct {
		v [10]*int
	}

	ac := acPool.Get()
	defer ac.Release()
	d := New[D](ac)

	for i := 0; i < len(d.v); i++ {
		d.v[i] = ac.Int(i)
		//d.v[i] = new(int) // this line makes this test failed.
		*d.v[i] = i
		runtime.GC()
	}

	for i, p := range d.v {
		if *p != i {
			t.Errorf("int %v is gced: %v", i, *p)
		}
	}
}

func Test_CheckNewMap(t *testing.T) {
	acPool.EnableDebugMode(true)
	ac := acPool.Get()
	defer ac.Release()

	defer func() {
		if err := recover(); err != nil {
			t.Errorf("faile to check")
		}
	}()

	type D struct {
		m map[int]*int
	}
	d := New[D](ac)
	d.m = NewMap[int, *int](ac, 0)
}

func Test_CheckExternalMap(t *testing.T) {
	acPool.EnableDebugMode(true)
	ac := acPool.Get()

	defer func() {
		if err := recover(); err == nil {
			t.Errorf("faile to check")
		}
	}()

	type D struct {
		m map[int]*int
	}
	d := New[D](ac)
	d.m = make(map[int]*int)

	ac.Release()
}

func Test_CheckExternalEnum(t *testing.T) {
	ac := acPool.Get()

	defer func() {
		if err := recover(); err == nil {
			t.Errorf("failed to check")
		}
	}()

	item := New[PbItem](ac)
	item.EnumVal = new(EnumA)
	ac.Release()
}

func Test_LacAsField(t *testing.T) {
	acPool.EnableDebugMode(true)
	ac := acPool.Get()
	defer ac.Release()

	defer func() {
		if err := recover(); err != nil {
			t.Errorf("failed to check")
		}
	}()

	type S struct {
		ac *Allocator
	}

	s := New[S](ac)
	s.ac = ac
}

func Test_ClosureAsField(t *testing.T) {
	acPool.EnableDebugMode(true)
	ac := acPool.Get()
	defer ac.Release()

	type S struct {
		c func() any
	}

	s := New[S](ac)
	s.c = Attach(ac, func() any {
		return s
	})

	runtime.GC()

	if s.c() != s {
		t.Fail()
	}
}

func Test_ExternalClosure(t *testing.T) {
	acPool.EnableDebugMode(true)
	ac := acPool.Get()

	type S struct {
		c func() any
	}

	s := New[S](ac)
	s.c = func() any {
		return s
	}

	defer func() {
		if e := recover(); e == nil {
			t.Errorf("should report unexported ptr")
		}
	}()

	ac.Release()
}

func Test_ShouldIgnoreFieldsOfMarkedExternal(t *testing.T) {
	acPool.EnableDebugMode(true)
	ac := acPool.Get()
	type D struct {
		i *int
	}
	type S struct {
		sub *D
	}
	s := New[S](ac)
	s.sub = Attach(ac, &D{i: new(int)})
	ac.Release()
}

func TestValidityCheck(t *testing.T) {
	ac := acPool.Get()
	ac.Release()
	defer func() {
		if err := recover(); err == nil {
			t.Errorf("should panic")
		}
	}()
	_ = New[int](ac)
}
