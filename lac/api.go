/*
 * Linear Allocator
 *
 * Improve the memory allocation and garbage collection performance.
 *
 * Copyright (C) 2020-2022 crazybie@github.com.
 * https://github.com/crazybie/linear_ac
 */

package lac

import (
	"fmt"
	"reflect"
	"sync/atomic"
	"unsafe"
)

// BuildInAc switches to native allocator.
var BuildInAc = &Allocator{disabled: true}

var acPool = Pool[*Allocator]{
	New: newLac,
}

func Get() *Allocator {
	return acPool.Get()
}

func (ac *Allocator) Release() {
	if ac == BuildInAc {
		return
	}
	ac.reset()
	acPool.Put(ac)
}

//IncRef should be used at outside the new goroutine, e.g.
//
// 	ac.IncRef() // <- should be called outside the new goroutine.
//  go func(){
// 		defer ac.DecRef()
//		....
//	}{}
//
// not in the new goroutine, otherwise the new goroutine maybe delayed after the caller quit,
// which may cause a UseAfterFree error.
func (ac *Allocator) IncRef() {
	atomic.AddInt32(&ac.refCnt, 1)
}

func (ac *Allocator) DecRef() {
	if atomic.AddInt32(&ac.refCnt, -1) <= 0 {
		ac.Release()
	}
}

//============================================================================
// allocation APIs
//============================================================================

func New[T any](ac *Allocator) (r *T) {
	if ac == nil || ac.disabled {
		return new(T)
	}
	return ac.typedAlloc(reflect.TypeOf((*T)(nil)), unsafe.Sizeof(*r), true).(*T)
}

func NewFrom[T any](ac *Allocator, v *T) *T {
	from := noEscape(v).(*T)
	if ac == nil || ac.disabled {
		// since the v is stack allocated due to noEscape, migrate it to heap.
		r := new(T)
		*r = *v
		return r
	}
	ret := ac.typedAlloc(reflect.TypeOf((*T)(nil)), unsafe.Sizeof(*from), false).(*T)
	memmoveNoHeapPointers(data(ret), unsafe.Pointer(from), unsafe.Sizeof(*from))
	return ret
}

func NewEnum[T any](ac *Allocator, e T) *T {
	if ac == nil || ac.disabled {
		r := new(T)
		*r = e
		return r
	}
	r := ac.typedAlloc(reflect.TypeOf((*T)(nil)), unsafe.Sizeof(e), false).(*T)
	*r = e
	return r
}

func NewSlice[T any](ac *Allocator, len, cap int) (r []T) {
	if ac == nil || ac.disabled {
		return make([]T, len, cap)
	}

	if len > cap {
		panic(fmt.Errorf("NewSlice: cap out of range"))
	}

	slice := (*sliceHeader)(unsafe.Pointer(&r))
	var t T
	slice.Data = ac.alloc(cap*int(unsafe.Sizeof(t)), false)
	slice.Len = len
	slice.Cap = cap
	return r
}

func NewMap[K comparable, V any](ac *Allocator, cap int) map[K]V {
	m := make(map[K]V, cap)
	if ac == nil || ac.disabled {
		return m
	}
	ac.keepAlive(m)
	return m
}

// AttachExternal can attach lac objects as well with no side effects.
func AttachExternal[T any](ac *Allocator, ptr T) T {
	if ac == nil || ac.disabled {
		return ptr
	}
	ac.keepAlive(ptr)
	return ptr
}

func Append[T any](ac *Allocator, s []T, v T) []T {
	if ac == nil || ac.disabled {
		return append(s, v)
	}

	h := (*sliceHeader)(unsafe.Pointer(&s))
	elemSz := int(unsafe.Sizeof(v))
	// grow
	if h.Len >= h.Cap {
		pre := *h
		h.Cap *= 2
		if h.Cap == 0 {
			h.Cap = 4
		}
		h.Data = ac.alloc(h.Cap*elemSz, false)
		memmoveNoHeapPointers(h.Data, pre.Data, uintptr(pre.Len*elemSz))
	}
	// append
	if h.Len < h.Cap {
		memmoveNoHeapPointers(unsafe.Add(h.Data, elemSz*h.Len), unsafe.Pointer(&v), uintptr(elemSz))
		h.Len++
	}
	return s
}

func (ac *Allocator) NewString(v string) string {
	if ac == nil || ac.disabled {
		return v
	}
	h := (*stringHeader)(unsafe.Pointer(&v))
	ptr := ac.alloc(h.Len, false)
	memmoveNoHeapPointers(ptr, h.Data, uintptr(h.Len))
	h.Data = ptr
	return v
}

//============================================================================
// Protobuf APIs
//============================================================================

func (ac *Allocator) Bool(v bool) (r *bool) {
	if ac == nil || ac.disabled {
		r = new(bool)
	} else {
		r = ac.typedAlloc(boolPtrType, unsafe.Sizeof(v), false).(*bool)
	}
	*r = v
	return
}

func (ac *Allocator) Int(v int) (r *int) {
	if ac == nil || ac.disabled {
		r = new(int)
	} else {
		r = ac.typedAlloc(intPtrType, unsafe.Sizeof(v), false).(*int)
	}
	*r = v
	return
}

func (ac *Allocator) Int32(v int32) (r *int32) {
	if ac == nil || ac.disabled {
		r = new(int32)
	} else {
		r = ac.typedAlloc(i32PtrType, unsafe.Sizeof(v), false).(*int32)
	}
	*r = v
	return
}

func (ac *Allocator) Uint32(v uint32) (r *uint32) {
	if ac == nil || ac.disabled {
		r = new(uint32)
	} else {
		r = ac.typedAlloc(u32PtrType, unsafe.Sizeof(v), false).(*uint32)
	}
	*r = v
	return
}

func (ac *Allocator) Int64(v int64) (r *int64) {
	if ac == nil || ac.disabled {
		r = new(int64)
	} else {
		r = ac.typedAlloc(i64PtrType, unsafe.Sizeof(v), false).(*int64)
	}
	*r = v
	return
}

func (ac *Allocator) Uint64(v uint64) (r *uint64) {
	if ac == nil || ac.disabled {
		r = new(uint64)
	} else {
		r = ac.typedAlloc(u64PtrType, unsafe.Sizeof(v), false).(*uint64)
	}
	*r = v
	return
}

func (ac *Allocator) Float32(v float32) (r *float32) {
	if ac == nil || ac.disabled {
		r = new(float32)
	} else {
		r = ac.typedAlloc(f32PtrType, unsafe.Sizeof(v), false).(*float32)
	}
	*r = v
	return
}

func (ac *Allocator) Float64(v float64) (r *float64) {
	if ac == nil || ac.disabled {
		r = new(float64)
	} else {
		r = ac.typedAlloc(f64PtrType, unsafe.Sizeof(v), false).(*float64)
	}
	*r = v
	return
}

func (ac *Allocator) String(v string) (r *string) {
	if ac == nil || ac.disabled {
		r = new(string)
		*r = v
	} else {
		r = ac.typedAlloc(strPtrType, unsafe.Sizeof(v), false).(*string)
		*r = ac.NewString(v)
	}
	return
}
