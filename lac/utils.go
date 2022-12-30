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
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"
)

var (
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

const PtrSize = int(unsafe.Sizeof(uintptr(0)))

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

//go:linkname memclrNoHeapPointers reflect.memclrNoHeapPointers
//go:noescape
func memclrNoHeapPointers(ptr unsafe.Pointer, n uintptr)

//go:linkname memmoveNoHeapPointers reflect.memmove
//go:noescape
func memmoveNoHeapPointers(to, from unsafe.Pointer, n uintptr)

// noEscape is to cheat the escape analyser to avoid heap alloc.
// NOTE:
// it's danger to make the must-escaped value as noEscape,
// for example mark a pointer returned from a function that points to the function's stack object as noEscape.
func noEscape(p any) (ret any) {
	*(*[2]uintptr)(unsafe.Pointer(&ret)) = *(*[2]uintptr)(unsafe.Pointer(&p))
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

func noMalloc(f func()) {
	checkMalloc(0, f)
}

func checkMalloc(max uint64, f func()) {
	var s, e runtime.MemStats
	runtime.ReadMemStats(&s)
	f()
	runtime.ReadMemStats(&e)
	if n := e.Mallocs - s.Mallocs; n > max {
		panic(fmt.Errorf("has %v malloc, bytes: %v", n, e.HeapAlloc-s.HeapAlloc))
	}
}

//============================================================================
// Pool
//============================================================================

// Pool pros over sync.Pool:
// 1. generic API
// 2. no boxing
// 3. support reserving in advance
type Pool[T any] struct {
	sync.Mutex
	New  func() T
	pool []T
}

func (p *Pool[T]) get() T {
	p.Lock()
	defer p.Unlock()
	if len(p.pool) == 0 {
		return p.New()
	}
	r := p.pool[len(p.pool)-1]
	p.pool = p.pool[:len(p.pool)-1]
	return r
}

func (p *Pool[T]) put(v T) {
	p.Lock()
	defer p.Unlock()
	p.pool = append(p.pool, v)
}

func (p *Pool[T]) clear() {
	p.Lock()
	defer p.Unlock()
	p.pool = nil
}

func (p *Pool[T]) reserve(cnt int) {
	p.Lock()
	defer p.Unlock()
	for i := 0; i < cnt; i++ {
		p.pool = append(p.pool, p.New())
	}
}

//============================================================================
// GoroutineId
//============================================================================

// https://notes.volution.ro/v1/2019/08/notes/23e3644e/

var goRoutineIdOffset uint64 = 0

func goRoutinePtr() unsafe.Pointer

func goRoutineId() uint64 {
	d := (*[32]uint64)(goRoutinePtr())
	if offset := atomic.LoadUint64(&goRoutineIdOffset); offset != 0 {
		return d[int(offset)]
	}
	id := goRoutineIdSlow()
	var n, offset int
	for idx, v := range d[:] {
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
