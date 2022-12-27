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
	"runtime"
	"testing"
)

func Test_CheckArray(t *testing.T) {
	DbgMode = true
	ac := BindNew()
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
	DbgMode = true
	ac := BindNew()

	type D struct {
		v []int
	}
	d := New[D](ac)
	d.v = NewSlice[int](ac, 1, 0)
	ac.Release()
}

func Test_CheckExternalSlice(t *testing.T) {
	DbgMode = true
	ac := BindNew()
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
	DbgMode = true
	ac := BindNew()
	defer func() {
		if err := recover(); err != nil {
			t.Errorf("faile to check")
		}
	}()

	type D struct {
		v []*int
	}
	d := New[D](ac)

	d.v = AttachExternalSlice(ac, make([]*int, 3))
	for i := 0; i < len(d.v); i++ {
		d.v[i] = AttachExternalPtr(ac, new(int))
		*d.v[i] = i
	}
	ac.Release()
}

func TestUseAfterFree_Pointer(t *testing.T) {
	DbgMode = true
	defer func() {
		DbgMode = false
		if err := recover(); err == nil {
			t.Errorf("failed to check")
		}
	}()
	ac := BindNew()
	d := New[PbData](ac)
	d.Age = ac.Int(11)
	ac.Release()
	if *d.Age == 11 {
		t.Errorf("not panic")
	}
}

func TestUseAfterFree_StructPointer(t *testing.T) {
	DbgMode = true
	defer func() {
		DbgMode = false
		if err := recover(); err == nil {
			t.Errorf("failed to check")
		}
	}()
	ac := BindNew()

	d := New[PbData](ac)
	d.InUse = New[PbItem](ac)

	ac.Release()
	c := *d.InUse
	t.Errorf("should panic")
	_ = c
}

func TestUseAfterFree_Slice(t *testing.T) {
	DbgMode = true
	defer func() {
		DbgMode = false
		if err := recover(); err == nil {
			t.Errorf("failed to check")
		}
	}()

	ac := BindNew()
	d := New[PbData](ac)
	ac.NewSlice(&d.Items, 1, 1)
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

	ac := BindNew()
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
	ac.Release()
}

func TestLinearAllocator_CheckNewMap(t *testing.T) {
	DbgMode = true
	ac := BindNew()
	defer func() {
		if err := recover(); err != nil {
			t.Errorf("faile to check")
		}
	}()

	type D struct {
		m map[int]*int
	}
	d := New[D](ac)
	d.m = NewMap[int, *int](ac)
	ac.Release()
}

func TestLinearAllocator_CheckExternalMap(t *testing.T) {
	DbgMode = true
	ac := BindNew()
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

func TestAllocator_CheckExternalEnum(t *testing.T) {
	ac := BindNew()
	defer func() {
		if err := recover(); err == nil {
			t.Errorf("failed to check")
		}
	}()

	item := New[PbItem](ac)
	item.EnumVal = new(EnumA)
	ac.Release()
}

func TestAllocator_AcAsField(t *testing.T) {
	DbgMode = true
	ac := BindNew()
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
	ac.Release()
}
