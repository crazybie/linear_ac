//
// Copyright (C) 2020-2021 crazybie@github.com.
//
//
// Linear Allocator
//
// Improve the memory allocation and garbage collection performance.
//
// https://github.com/crazybie/linear_ac
//

package lac

import (
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"
)

var (
	ptrSize = int(unsafe.Sizeof(uintptr(0)))

	boolPtrType = reflect.TypeOf((*bool)(nil))
	intPtrType  = reflect.TypeOf((*int)(nil))
	i32PtrType  = reflect.TypeOf((*int32)(nil))
	u32PtrType  = reflect.TypeOf((*uint32)(nil))
	i64PtrType  = reflect.TypeOf((*int64)(nil))
	u64PtrType  = reflect.TypeOf((*uint64)(nil))
	f32PtrType  = reflect.TypeOf((*float32)(nil))
	f64PtrType  = reflect.TypeOf((*float64)(nil))
	strPtrType  = reflect.TypeOf((*string)(nil))
)

type sliceHeader struct {
	Data unsafe.Pointer
	Len  int
	Cap  int
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
}

//go:linkname reflect_typedmemmove reflect.typedmemmove
func reflect_typedmemmove(typ, dst, src unsafe.Pointer)

//go:linkname reflect_memclrNoHeapPointers reflect.memclrNoHeapPointers
func reflect_memclrNoHeapPointers(ptr unsafe.Pointer, n uintptr)

//go:linkname reflect_memmove reflect.memmove
func reflect_memmove(to, from unsafe.Pointer, n uintptr)

//============================================================================
// GoroutineId
//============================================================================

// https://notes.volution.ro/v1/2019/08/notes/23e3644e/

var goRoutineIdOffset uint64 = 0

func goRoutinePtr() uint64

func goRoutineId() uint64 {
	data := (*[32]uint64)(unsafe.Pointer(uintptr(goRoutinePtr())))
	if offset := atomic.LoadUint64(&goRoutineIdOffset); offset != 0 {
		return data[int(offset)]
	}
	id := goRoutineIdSlow()
	var n, offset int
	for idx, v := range data[:] {
		if v == id {
			offset = idx
			n++
			if n >= 2 {
				break
			}
		}
	}
	if n == 1 {
		atomic.StoreUint64(&goRoutineIdOffset, uint64(offset))
	}
	return id
}

func goRoutineIdSlow() uint64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	stk := strings.TrimPrefix(string(buf[:n]), "goroutine ")
	if id, err := strconv.Atoi(strings.Fields(stk)[0]); err != nil {
		panic(err)
	} else {
		return uint64(id)
	}
}

// Helpers

func add(p unsafe.Pointer, offset int) unsafe.Pointer {
	return unsafe.Pointer(uintptr(p) + uintptr(offset))
}

//go:noinline
func forceStackSplit(i int) int {
	if i > 0 {
		return forceStackSplit(i - 1)
	}
	return i
}

//go:noinline
//go:nosplit
func noEscape(p interface{}) (ret interface{}) {
	r := *(*[2]uintptr)(unsafe.Pointer(&p))
	//forceStackSplit(1000)
	*(*[2]uintptr)(unsafe.Pointer(&ret)) = r
	return
}

func data(i interface{}) unsafe.Pointer {
	return (*emptyInterface)(unsafe.Pointer(&i)).Data
}

func interfaceOfUnexported(v reflect.Value) (ret interface{}) {
	v2 := (*reflectedValue)(unsafe.Pointer(&v))
	r := (*emptyInterface)(unsafe.Pointer(&ret))
	r.Type = v2.Type
	r.Data = v2.Ptr
	return
}

func copyBytes(src, dst unsafe.Pointer, len int) {
	reflect_memmove(dst, src, uintptr(len))
}

func clearBytes(dst unsafe.Pointer, len int) {
	reflect_memclrNoHeapPointers(dst, uintptr(len))
}

//============================================================================
// syncPool
//============================================================================

type syncPool[T any] struct {
	sync.Mutex
	New  func() *T
	pool []*T
}

func (p *syncPool[T]) get() *T {
	p.Lock()
	defer p.Unlock()
	if len(p.pool) == 0 {
		return p.New()
	}
	r := p.pool[len(p.pool)-1]
	p.pool = p.pool[:len(p.pool)-1]
	return r
}

func (p *syncPool[T]) put(v *T) {
	p.Lock()
	defer p.Unlock()
	p.pool = append(p.pool, v)
}

func (p *syncPool[T]) putMany(v []*T) {
	p.Lock()
	defer p.Unlock()
	for i := 0; i < len(v); i++ {
		p.pool = append(p.pool, v[i])
	}
}

func (p *syncPool[T]) clear() {
	p.Lock()
	defer p.Unlock()
	p.pool = nil
}

func (p *syncPool[T]) reserve(cnt int) {
	p.Lock()
	defer p.Unlock()
	for i := 0; i < cnt; i++ {
		p.pool = append(p.pool, p.New())
	}
}
