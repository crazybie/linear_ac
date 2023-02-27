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

const ptrSize = int(unsafe.Sizeof(uintptr(0)))

func init() {
	if ptrSize != 8 {
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

type number interface {
	~int8 | ~int16 | ~int | ~int32 | ~int64 |
		~uint8 | ~uint16 | ~uint | ~uint32 | ~uint64 |
		~float32 | ~float64
}

func max[T number](a, b T) T {
	if a > b {
		return a
	}
	return b
}

//============================================================================
// Spin lock
//============================================================================

type spinLock int32

func (s *spinLock) Lock() {
	for !atomic.CompareAndSwapInt32((*int32)(s), 0, 1) {
		runtime.Gosched()
	}
}

func (s *spinLock) Unlock() {
	atomic.StoreInt32((*int32)(s), 0)
}

//============================================================================
// weakUniqQueue
//============================================================================

// weakUniqQueue is used to reduce the duplication of elems in queue.
// the major purpose is to reduce memory usage.
type weakUniqQueue[T any] struct {
	spinLock
	slice     []T
	uniqRange int
	equal     func(a, b T) bool
}

func newWeakUniqQueue[T any](uniqRange int, eq func(a, b T) bool) weakUniqQueue[T] {
	return weakUniqQueue[T]{equal: eq, uniqRange: uniqRange}
}

func (e *weakUniqQueue[T]) Clear() {
	e.Lock()
	defer e.Unlock()
	e.slice = nil
}

func (e *weakUniqQueue[T]) Put(a T) {
	e.Lock()
	defer e.Unlock()
	if l := len(e.slice); l > 0 {
		if l < e.uniqRange {
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

func anyEq(a, b any) bool {
	return a == b
}

func eq[T comparable](a, b T) bool {
	return a == b
}
