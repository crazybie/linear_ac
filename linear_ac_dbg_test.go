package linear_ac

import (
	"runtime"
	"testing"
)

func Test_CheckArray(t *testing.T) {
	DbgMode = true
	ac := BindAc()
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
	ac.Release()
}

func Test_CheckInternalSlice(t *testing.T) {
	DbgMode = true
	ac := BindAc()

	type D struct {
		v []int
	}
	var d *D
	ac.New(&d)
	ac.NewSlice(&d.v, 1, 0)
	ac.Release()
}

func Test_CheckExternalSlice(t *testing.T) {
	DbgMode = true
	ac := BindAc()
	defer func() {
		if err := recover(); err == nil {
			t.Errorf("faile to check")
		}
	}()

	type D struct {
		v []*int
	}
	var d *D
	ac.New(&d)

	d.v = make([]*int, 3)
	for i := 0; i < len(d.v); i++ {
		d.v[i] = new(int)
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
	ac := BindAc()
	var d *PbData
	ac.New(&d)
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
	ac := BindAc()

	var d *PbData
	ac.New(&d)
	var item *PbItem
	ac.New(&item)
	d.InUse = item

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

	ac := BindAc()
	var d *PbData
	ac.New(&d)
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

	ac := BindAc()

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
	ac.Release()
}

func TestLinearAllocator_ExternalMap(t *testing.T) {
	DbgMode = true
	ac := BindAc()
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
	ac.Release()
}

func TestAllocator_CheckExternalEnum(t *testing.T) {
	ac := BindAc()
	defer func() {
		if err := recover(); err == nil {
			t.Errorf("failed to check")
		}
	}()

	var item *PbItem
	ac.New(&item)
	item.EnumVal = new(EnumA)
	ac.Release()
}
