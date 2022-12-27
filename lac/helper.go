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
	"reflect"
	"sync"
	"sync/atomic"
	"unsafe"
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
	atomic.AddInt32(&ac.refCnt, 1)
}

func (ac *Allocator) DecRef() {
	if atomic.AddInt32(&ac.refCnt, -1) <= 0 {
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
	return ac.typedNew(reflect.TypeOf((*T)(nil)), true).(*T)
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

func AttachExternal[T any](ac *Allocator, ptr T) T {
	ac.keepAlive(ptr)
	return ptr
}

// Append works with no malloc, but caller side has weired malloc.
// prefer the no-generic version: SliceAppend.
func Append[T any](ac *Allocator, s []T, v T) []T {

	if ac.disabled {
		return append(s, v)
	}

	header := (*sliceHeader)(unsafe.Pointer(&s))
	elemSz := int(unsafe.Sizeof(v))

	// grow
	if header.Len >= header.Cap {
		pre := *header
		if header.Cap >= 16 {
			header.Cap = int(float32(header.Cap) * 1.5)
		} else {
			header.Cap *= 2
		}
		if header.Cap == 0 {
			header.Cap = 1
		}
		header.Data = ac.alloc(header.Cap*elemSz, false)
		copyBytes(pre.Data, header.Data, pre.Len*elemSz)
	}

	// append
	if header.Len < header.Cap {
		dst := add(header.Data, elemSz*header.Len)
		copyBytes(unsafe.Pointer(&v), dst, elemSz)
		header.Len++
	}
	return s
}
