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
	"reflect"
	"unsafe"
)

var (
	DisableAllLac = false

	// our memory is much cheaper than systems,
	// so we can be more aggressive than `append`.
	SliceExtendRatio = 2.5

	BugfixClearPointerSlice = true
)

func (p *AllocatorPool) Get() *Allocator {
	if p == nil || DisableAllLac {
		return nil
	}
	ac := p.Pool.Get()
	ac.valid = true
	return ac
}

func (ac *Allocator) Release() {
	if ac == nil || ac.disabled {
		return
	}
	ac.reset()
	ac.acPool.Put(ac)
}

// IncRef should be called before and outside the new goroutine, never be in the new goroutine,
// otherwise the execution of new goroutine may be delayed after the caller quit,
// which may cause a UseAfterFree error. e.g.
//
//	ac.IncRef() // <<<- Correct usage. should be called before and outside the new goroutine.
//	go func() {
//
//		// ac.IncRef() <<<- !!!!!!! Incorrect usage !!!!!!!!!!!!
//
//		defer ac.DecRef()
//		....
//	}()
//
// if IncRef is not call correctly the Lac will be recycled ahead of time,
// in debug mode your Lac allocated objects become corrupted and panic occurs when using them.
// UseAfterFree can also be caught by the validity check in release mode.
//
// This appointment also ensure the single-threaded version will never run in parallel with
// the multi-threaded version.
func (ac *Allocator) IncRef() {
	if ac == nil || ac.disabled {
		return
	}
	ac.checkValidity()
	ac.refCnt.Add(1)
}

// DecRef will put the ac back into Pool if ref count reduced to zero.
// If one DecRef call is missed causes the Lac not go back to Pool, it will be recycled by GC later.
// If more DecRef calls are called cause the ref cnt reduced to negative, panic in debug mode.
func (ac *Allocator) DecRef() {
	if ac == nil || ac.disabled {
		return
	}
	ac.checkValidity()
	if n := ac.refCnt.Add(-1); n <= 0 {
		if n < 0 {
			errorf(ac.acPool, "potential bug: ref cnt is negative: %v", n)
		}
		ac.Release()
	}
}

//============================================================================
// Allocation APIs
//============================================================================

func New[T any](ac *Allocator) (r *T) {
	if ac == nil || ac.disabled {
		return new(T)
	}

	r = (*T)(ac.alloc(int(unsafe.Sizeof(*r)), true))
	if ac.acPool.debugMode {
		if reflect.TypeOf(r).Elem().Kind() == reflect.Struct {
			ac.debugScan(r)
		}
	}
	return r
}

// NewFrom copy the src object from heap to lac thus slower than New due to the heap malloc of src.
// **Prefer using New for better performance**.
// It is useful for old-code migration using struct literal syntax:
//
//	obj := lac.NewFrom(ac, &SomeData{
//		Field1: Value1,
//		Field2: Value2,
//	})
//
// This is a bit clearer than the following `new` syntax:
//
//	obj := lac.New[SomeData](ac)
//	obj.Field1 = Value1
//	obj.Field2 = Value2
func NewFrom[T any](ac *Allocator, src *T) *T {
	if ac == nil || ac.disabled {
		// NOTE:
		// we should not use `noescape` to avoid heap alloc the src here,
		// because it will cause all sub fields of src be stack allocated,
		// and the memmove only copy the top level fields,
		// therefor cause all sub pointer fields become dangled.
		return src
	}

	sz := unsafe.Sizeof(*src)
	// safe to avoid zeroing the memory because no wb can be triggered here.
	ret := (*T)(ac.alloc(int(sz), false))
	memmoveNoHeapPointers(unsafe.Pointer(ret), unsafe.Pointer(src), sz)

	if ac.acPool.debugMode {
		if reflect.TypeOf(ret).Elem().Kind() == reflect.Struct {
			ac.debugScan(ret)
		}
	}
	return ret
}

// NewSlice does not zero the slice automatically, this is OK with most cases and can improve the performance.
// zero it yourself for your need.
func NewSlice[T any](ac *Allocator, len, cap int) (r []T) {
	if ac == nil || ac.disabled {
		return make([]T, len, cap)
	}

	// keep same with systems `new`.
	if len > cap {
		panic("NewSlice: cap out of range")
	}

	slice := (*sliceHeader)(unsafe.Pointer(&r))
	var t T
	// FIX: rubbish in the slice may cause panic in the write barrier.
	zero := mayContainsPtr(reflect.TypeOf(t).Kind())
	if !BugfixClearPointerSlice {
		zero = false
	}
	slice.Data = ac.alloc(cap*int(unsafe.Sizeof(t)), zero)
	slice.Len = int64(len)
	slice.Cap = int64(cap)
	return r
}

func Append[T any](ac *Allocator, s []T, elems ...T) []T {
	if ac == nil || ac.disabled {
		return append(s, elems...)
	}

	if len(elems) == 0 {
		return s
	}

	h := (*sliceHeader)(unsafe.Pointer(&s))
	elemSz := int(unsafe.Sizeof(elems[0]))

	// grow
	if h.Len >= h.Cap {
		pre := *h

		cur := float64(h.Cap)
		h.Cap = max(int64(cur*SliceExtendRatio), pre.Cap+int64(len(elems)))
		// prefer to fit in a normal chunk.
		if h.Cap > int64(ac.acPool.chunkPool.ChunkSize) && SliceExtendRatio > 1.5 {
			small := int64(cur * 1.5)
			if small > pre.Cap+int64(len(elems)) {
				h.Cap = small
			}
		}

		if h.Cap < 16 {
			h.Cap = 16
		}

		sz := int(h.Cap) * elemSz
		h.Data = ac.alloc(sz, false)
		memmoveNoHeapPointers(h.Data, pre.Data, uintptr(int(pre.Len)*elemSz))

		// clear the reset part

		// FIX: rubbish in the slice may cause panic in the write barrier.
		var t T
		zero := mayContainsPtr(reflect.TypeOf(t).Kind())
		if !BugfixClearPointerSlice {
			zero = false
		}
		if zero {
			used := elemSz * int(pre.Len)
			memclrNoHeapPointers(unsafe.Add(h.Data, used), uintptr(sz-used))
		}
	}

	// append
	src := (*sliceHeader)(unsafe.Pointer(&elems))
	memmoveNoHeapPointers(unsafe.Add(h.Data, elemSz*int(h.Len)), src.Data, uintptr(elemSz*int(src.Len)))
	h.Len += src.Len

	return s
}

func NewMap[K comparable, V any](ac *Allocator, cap int) map[K]V {
	m := make(map[K]V, cap)
	if ac == nil || ac.disabled {
		return m
	}
	ac.keepAlive(m)
	return m
}

func NewEnum[T any](ac *Allocator, e T) *T {
	if ac == nil || ac.disabled {
		r := new(T)
		*r = e
		return r
	}
	r := (*T)(ac.alloc(int(unsafe.Sizeof(e)), false))
	*r = e
	return r
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

// Attach mark ptr as external pointer and will keep ptr alive during GC,
// otherwise the ptr from heap may be GCed and cause a dangled pointer, no panic will report by the runtime.
// So make sure to mark objects from native heap as external pointers by using this function.
// External pointers will be checked in debug mode.
// Can attach Lac objects as well without any side effects.
//
// NOTE: you must attach the external ptr **before** assigning it to the lac-allocated object,
// this is to ensure the external ptr is always reachable for the GC. usage:
// ```go
//
//	obj := lac.New[ObjType](ac)
//	obj.Field = lac.Attach(ac, externalPtr)
//
// ```
func Attach[T any](ac *Allocator, ptr T) T {
	if ac == nil || ac.disabled {
		return ptr
	}
	ac.keepAlive(ptr)
	return ptr
}

//============================================================================
// Protobuf2 APIs
//============================================================================

func (ac *Allocator) Bool(v bool) (r *bool) {
	if ac == nil || ac.disabled {
		r = new(bool)
	} else {
		r = (*bool)(ac.alloc(int(unsafe.Sizeof(v)), false))
	}
	*r = v
	return
}

func (ac *Allocator) Int(v int) (r *int) {
	if ac == nil || ac.disabled {
		r = new(int)
	} else {
		r = (*int)(ac.alloc(int(unsafe.Sizeof(v)), false))
	}
	*r = v
	return
}

func (ac *Allocator) Int32(v int32) (r *int32) {
	if ac == nil || ac.disabled {
		r = new(int32)
	} else {
		r = (*int32)(ac.alloc(int(unsafe.Sizeof(v)), false))
	}
	*r = v
	return
}

func (ac *Allocator) Uint32(v uint32) (r *uint32) {
	if ac == nil || ac.disabled {
		r = new(uint32)
	} else {
		r = (*uint32)(ac.alloc(int(unsafe.Sizeof(v)), false))
	}
	*r = v
	return
}

func (ac *Allocator) Int64(v int64) (r *int64) {
	if ac == nil || ac.disabled {
		r = new(int64)
	} else {
		r = (*int64)(ac.alloc(int(unsafe.Sizeof(v)), false))
	}
	*r = v
	return
}

func (ac *Allocator) Uint64(v uint64) (r *uint64) {
	if ac == nil || ac.disabled {
		r = new(uint64)
	} else {
		r = (*uint64)(ac.alloc(int(unsafe.Sizeof(v)), false))
	}
	*r = v
	return
}

func (ac *Allocator) Float32(v float32) (r *float32) {
	if ac == nil || ac.disabled {
		r = new(float32)
	} else {
		r = (*float32)(ac.alloc(int(unsafe.Sizeof(v)), false))
	}
	*r = v
	return
}

func (ac *Allocator) Float64(v float64) (r *float64) {
	if ac == nil || ac.disabled {
		r = new(float64)
	} else {
		r = (*float64)(ac.alloc(int(unsafe.Sizeof(v)), false))
	}
	*r = v
	return
}

func (ac *Allocator) String(v string) (r *string) {
	if ac == nil || ac.disabled {
		r = new(string)
		*r = v
	} else {
		r = (*string)(ac.alloc(int(unsafe.Sizeof(v)), false))
		*r = ac.NewString(v)
	}
	return
}
