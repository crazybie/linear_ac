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
	"sync"
)

// BuildInAc switches to native allocator.
var BuildInAc = &Allocator{disabled: true}

// Bind allocator to goroutine

var acMap = sync.Map{}

var acPool = syncPool[Allocator]{
	New: newLac,
}

func Get() *Allocator {
	return acPool.get()
}

func BindNew() *Allocator {
	ac := acPool.get()
	acMap.Store(goRoutineId(), ac)
	return ac
}

func BindGet() *Allocator {
	if val, ok := acMap.Load(goRoutineId()); ok {
		return val.(*Allocator)
	}
	return BuildInAc
}

func (ac *Allocator) Unbind() {
	if BindGet() == ac {
		acMap.Delete(goRoutineId())
	}
}

func (ac *Allocator) Release() {
	if ac == BuildInAc {
		return
	}
	ac.Unbind()
	ac.reset()
	acPool.put(ac)
}

func (ac *Allocator) IncRef() {
	ac.refCnt++
}

func (ac *Allocator) DecRef() {
	ac.refCnt--
	if ac.refCnt <= 0 {
		ac.Release()
	}
}

func (ac *Allocator) Bool(v bool) (r *bool) {
	if ac.disabled {
		r = new(bool)
	} else {
		r = ac.typedNew(boolPtrType, false).(*bool)
	}
	*r = v
	return
}

func (ac *Allocator) Int(v int) (r *int) {
	if ac.disabled {
		r = new(int)
	} else {
		r = ac.typedNew(intPtrType, false).(*int)
	}
	*r = v
	return
}

func (ac *Allocator) Int32(v int32) (r *int32) {
	if ac.disabled {
		r = new(int32)
	} else {
		r = ac.typedNew(i32PtrType, false).(*int32)
	}
	*r = v
	return
}

func (ac *Allocator) Uint32(v uint32) (r *uint32) {
	if ac.disabled {
		r = new(uint32)
	} else {
		r = ac.typedNew(u32PtrType, false).(*uint32)
	}
	*r = v
	return
}

func (ac *Allocator) Int64(v int64) (r *int64) {
	if ac.disabled {
		r = new(int64)
	} else {
		r = ac.typedNew(i64PtrType, false).(*int64)
	}
	*r = v
	return
}

func (ac *Allocator) Uint64(v uint64) (r *uint64) {
	if ac.disabled {
		r = new(uint64)
	} else {
		r = ac.typedNew(u64PtrType, false).(*uint64)
	}
	*r = v
	return
}

func (ac *Allocator) Float32(v float32) (r *float32) {
	if ac.disabled {
		r = new(float32)
	} else {
		r = ac.typedNew(f32PtrType, false).(*float32)
	}
	*r = v
	return
}

func (ac *Allocator) Float64(v float64) (r *float64) {
	if ac.disabled {
		r = new(float64)
	} else {
		r = ac.typedNew(f64PtrType, false).(*float64)
	}
	*r = v
	return
}

func (ac *Allocator) String(v string) (r *string) {
	if ac.disabled {
		r = new(string)
		*r = v
	} else {
		r = ac.typedNew(strPtrType, false).(*string)
		*r = ac.NewString(v)
	}
	return
}

//--------------------------------------
// generic APIs
//--------------------------------------

func New[T any](ac *Allocator) *T {
	var r *T
	ac.New(&r)
	return r
}

func NewCopy[T any](ac *Allocator, from *T) *T {
	return ac.NewCopy(from).(*T)
}

func NewEnum[T any](ac *Allocator, e T) *T {
	return ac.Enum(e).(*T)
}

func NewSlice[T any](ac *Allocator, len, cap int) []T {
	var r []T
	ac.NewSlice(&r, len, cap)
	return r
}

func NewMap[K comparable, V any](ac *Allocator) map[K]V {
	var r map[K]V
	ac.NewMap(&r)
	return r
}

// ExternalPtr stores the pointer on the function stack to prevent the gc sweep it.
func ExternalPtr[T any](ac *Allocator, ptr T) T {
	ac.keepAlive(ptr)
	return ptr
}
