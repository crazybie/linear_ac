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
	"runtime"
	"sync/atomic"
	"unsafe"
)

const PtrSize = int(unsafe.Sizeof(uintptr(0)))

func init() {
	if PtrSize != 8 {
		panic("expect 64bit platform")
	}
}

type sliceHeader struct {
	Data unsafe.Pointer
	Len  int64
	Cap  int64
}

type stringHeader struct {
	Data unsafe.Pointer
	Len  int
}

type emptyInterface struct {
	Type unsafe.Pointer
	Data unsafe.Pointer
}

type reflectedValue struct {
	Type unsafe.Pointer
	Ptr  unsafe.Pointer
	flag uintptr
}

const (
	flagIndir uintptr = 1 << 7
)

//go:linkname memclrNoHeapPointers reflect.memclrNoHeapPointers
//go:noescape
func memclrNoHeapPointers(ptr unsafe.Pointer, n uintptr)

//go:linkname memmoveNoHeapPointers reflect.memmove
//go:noescape
func memmoveNoHeapPointers(to, from unsafe.Pointer, n uintptr)

func data(i interface{}) unsafe.Pointer {
	return (*emptyInterface)(unsafe.Pointer(&i)).Data
}

func interfaceOfUnexported(v reflect.Value) (ret interface{}) {
	v2 := (*reflectedValue)(unsafe.Pointer(&v))
	r := (*emptyInterface)(unsafe.Pointer(&ret))
	r.Type = v2.Type
	switch {
	case v2.flag&flagIndir != 0:
		r.Data = *(*unsafe.Pointer)(v2.Ptr)
	default:
		r.Data = v2.Ptr
	}
	return
}

func interfaceEqual(a, b any) bool {
	return *(*emptyInterface)(unsafe.Pointer(&a)) == *(*emptyInterface)(unsafe.Pointer(&b))
}

func resetSlice[T any](s []T) []T {
	c := cap(s)
	s = s[:c]
	var zero T
	for i := 0; i < c; i++ {
		s[i] = zero
	}
	return s[:0]
}

//============================================================================
// Spin lock
//============================================================================

type SpinLock int32

func (s *SpinLock) Lock() {
	for !atomic.CompareAndSwapInt32((*int32)(s), 0, 1) {
		runtime.Gosched()
	}
}

func (s *SpinLock) Unlock() {
	atomic.StoreInt32((*int32)(s), 0)
}

//============================================================================
// WeakUniqQueue
//============================================================================

// WeakUniqQueue is used to reduce the duplication of elems in queue.
// the major purpose is to reduce memory usage.
type WeakUniqQueue[T any] struct {
	SpinLock
	slice           []T
	strongUniqRange int
	equal           func(a, b T) bool
}

func NewWeakUniqQueue[T any](strongUniqRange int, eq func(a, b T) bool) WeakUniqQueue[T] {
	return WeakUniqQueue[T]{equal: eq, strongUniqRange: strongUniqRange}
}

func (e *WeakUniqQueue[T]) Clear() {
	e.slice = nil
}

func (e *WeakUniqQueue[T]) Put(a T) {
	e.Lock()
	defer e.Unlock()
	if l := len(e.slice); l > 0 {
		if l < e.strongUniqRange {
			for _, k := range e.slice {
				if e.equal(k, a) {
					return
				}
			}
		}
		last := e.slice[l-1]
		if e.equal(a, last) {
			return
		}
	}
	e.slice = append(e.slice, a)
}

func unsafePtrEq(a, b unsafe.Pointer) bool {
	return a == b
}

func anyEq(a, b any) bool {
	return a == b
}
