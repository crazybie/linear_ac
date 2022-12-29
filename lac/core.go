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
	"sync"
	"unsafe"
)

var (
	DbgMode         = false
	DisableLinearAc = false
	ChunkSize       = 1024 * 8
)

// Chunk

type chunk []byte

var chunkPool = syncPool[chunk]{
	New: func() *chunk {
		ck := make(chunk, 0, ChunkSize)
		return &ck
	},
}

// Objects in sync.Pool will be recycled on demand by the system (usually after two full-gc).
// we can put chunks here to make pointers live longer,
// useful to diagnosis use-after-free bugs.
var diagnosisChunkPool = sync.Pool{}

func init() {
	if DbgMode {
		ChunkSize /= 8
	}
}

// Allocator

type Allocator struct {
	sync.Mutex

	disabled    bool
	chunks      []*chunk
	curChunk    int
	refCnt      int32
	dbgScanObjs []interface{}

	externalPtr   []unsafe.Pointer
	externalSlice []unsafe.Pointer
	externalMap   []interface{}
}

func newLac() *Allocator {
	ac := &Allocator{
		disabled: DisableLinearAc,
		refCnt:   1,
		chunks:   make([]*chunk, 0, 1),
	}
	return ac
}

func (ac *Allocator) keepAlive(ptr interface{}) {
	if ac.disabled {
		return
	}

	d := data(ptr)
	if d == nil {
		return
	}

	switch reflect.TypeOf(ptr).Kind() {
	case reflect.Ptr:
		ac.externalPtr = append(ac.externalPtr, d)
	case reflect.Slice:
		ac.externalSlice = append(ac.externalSlice, unsafe.Pointer((*reflect.SliceHeader)(d).Data))
	case reflect.Map:
		ac.externalMap = append(ac.externalMap, d)
	default:
		panic(fmt.Errorf("unsupported type %v", reflect.TypeOf(ptr).String()))
	}
}

func (ac *Allocator) reset() {
	if ac.disabled {
		return
	}

	if DbgMode {
		ac.debugCheck(true)
		ac.dbgScanObjs = ac.dbgScanObjs[:0]
	}

	for _, ck := range ac.chunks {
		*ck = (*ck)[:0]
		if DbgMode {
			diagnosisChunkPool.Put(ck)
		} else {
			// only reuse the normal chunks,
			// otherwise we may have too many large chunks wasted.
			if cap(*ck) == ChunkSize {
				chunkPool.put(ck)
			}
		}
	}
	// clear all ref
	ac.chunks = ac.chunks[:cap(ac.chunks)]
	for i := 0; i < cap(ac.chunks); i++ {
		ac.chunks[i] = nil
	}
	ac.chunks = ac.chunks[:0]
	ac.curChunk = 0

	// clear externals
	ac.externalPtr = ac.externalPtr[:0]
	ac.externalSlice = ac.externalSlice[:0]
	ac.externalMap = ac.externalMap[:0]

	ac.disabled = DisableLinearAc
	ac.refCnt = 1
}

func (ac *Allocator) typedNew(ptrTp reflect.Type, sz uintptr, zero bool) (ret interface{}) {
	if sz == 0 {
		sz = ptrTp.Elem().Size()
	}
	ptr := ac.alloc(int(sz), zero)
	*(*emptyInterface)(unsafe.Pointer(&ret)) = emptyInterface{data(ptrTp), ptr}

	if DbgMode {
		if ptrTp.Elem().Kind() == reflect.Struct {
			ac.dbgScanObjs = append(ac.dbgScanObjs, ret)
		}
	}

	return
}

func (ac *Allocator) alloc(need int, zero bool) unsafe.Pointer {
	// shared with other goroutines?
	if ac.refCnt > 1 {
		ac.Lock()
		defer ac.Unlock()
	}

	if len(ac.chunks) == 0 {
		ac.chunks = append(ac.chunks, chunkPool.get())
	}

	aligned := (need + PtrSize + 1) & ^(PtrSize - 1)

start:
	cur := ac.chunks[ac.curChunk]
	used := len(*cur)

	if used+aligned > cap(*cur) {

		if ac.curChunk == len(ac.chunks)-1 {
			// if we get to the end of the chunk list,
			// we enqueue a new one the end of it.
			var ck *chunk
			if aligned > ChunkSize {
				// recreate a large chunk
				c := make(chunk, 0, aligned)
				ck = &c
			} else {
				ck = chunkPool.get()
			}
			ac.chunks = append(ac.chunks, ck)
		} else if cap(*ac.chunks[ac.curChunk+1]) < aligned {
			// if the next normal chunk is still under required size,
			// recreate a large one and replace it.
			chunkPool.put(ac.chunks[ac.curChunk+1])
			ck := make(chunk, 0, aligned)
			ac.chunks[ac.curChunk+1] = &ck
		}

		ac.curChunk++
		goto start
	}

	*cur = (*cur)[:used+aligned]
	base := unsafe.Pointer((*reflect.SliceHeader)(unsafe.Pointer(cur)).Data)
	ptr := unsafe.Add(base, used)
	if zero {
		memclrNoHeapPointers(ptr, uintptr(aligned))
	}
	return ptr
}

func (ac *Allocator) New(ptrToPtr interface{}) {
	tmp := noEscape(ptrToPtr)

	if ac.disabled {
		tp := reflect.TypeOf(tmp).Elem().Elem()
		reflect.ValueOf(tmp).Elem().Set(reflect.New(tp))
		return
	}

	tp := reflect.TypeOf(tmp).Elem()
	v := ac.typedNew(tp, 0, true)
	reflect.ValueOf(tmp).Elem().Set(reflect.ValueOf(v))
}

// NewFrom is useful for code migration.
// it is a bit slower than New() due to the src object construction and additional memory move from stack to heap.
func (ac *Allocator) NewFrom(src interface{}) (ret interface{}) {
	ptrTemp := noEscape(src)
	ptrType := reflect.TypeOf(ptrTemp)
	tp := ptrType.Elem()

	if ac.disabled {
		return ptrTemp
	} else {
		ret = ac.typedNew(ptrType, 0, false)
		memmove(data(ret), data(ptrTemp), tp.Size())
	}
	return
}

func (ac *Allocator) NewString(v string) string {
	if ac.disabled {
		return v
	}
	h := (*reflect.StringHeader)(unsafe.Pointer(&v))
	ptr := ac.alloc(h.Len, false)
	memmove(ptr, unsafe.Pointer(h.Data), uintptr(h.Len))
	h.Data = uintptr(ptr)
	return v
}

// NewMap use build-in allocator
func (ac *Allocator) NewMap(mapPtr interface{}) {
	mapPtrTemp := noEscape(mapPtr)

	if ac.disabled {
		tp := reflect.TypeOf(mapPtrTemp).Elem()
		reflect.ValueOf(mapPtrTemp).Elem().Set(reflect.MakeMap(tp))
		return
	}

	m := reflect.MakeMap(reflect.TypeOf(mapPtrTemp).Elem())
	reflect.ValueOf(mapPtrTemp).Elem().Set(m)
	ac.keepAlive(m.Interface())
}

func (ac *Allocator) NewSlice(slicePtr interface{}, len, cap int) {
	slicePtrTmp := noEscape(slicePtr)

	if ac.disabled {
		v := reflect.MakeSlice(reflect.TypeOf(slicePtrTmp).Elem(), len, cap)
		reflect.ValueOf(slicePtrTmp).Elem().Set(v)
		return
	}

	slicePtrType := reflect.TypeOf(slicePtrTmp)
	if slicePtrType.Kind() != reflect.Ptr || slicePtrType.Elem().Kind() != reflect.Slice {
		panic("need a pointer to slice")
	}

	slice := (*reflect.SliceHeader)(data(slicePtrTmp))
	if cap < len {
		cap = len
	}
	slice.Data = uintptr(ac.alloc(cap*int(slicePtrType.Elem().Elem().Size()), false))
	slice.Len = len
	slice.Cap = cap
}

// CopySlice is useful to create simple slice (simple type as element).
func (ac *Allocator) CopySlice(slice interface{}) (ret interface{}) {
	sliceTmp := noEscape(slice)
	if ac.disabled {
		return sliceTmp
	}

	sliceType := reflect.TypeOf(sliceTmp)
	if sliceType.Kind() != reflect.Slice {
		panic("need a slice")
	}
	elemType := sliceType.Elem()
	switch elemType.Kind() {
	case reflect.Int, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
	default:
		panic("must be simple type")
	}

	// input is a temp copy, directly use it.
	ret = sliceTmp
	header := (*reflect.SliceHeader)(data(sliceTmp))
	size := int(elemType.Size()) * header.Len
	dst := ac.alloc(size, false)
	memmove(dst, unsafe.Pointer(header.Data), uintptr(size))
	header.Data = uintptr(dst)

	runtime.KeepAlive(slice)
	return ret
}

// SliceAppend with no malloc.
// NOTE: the generic version has weird malloc thus not preferred in extreme case.
func (ac *Allocator) SliceAppend(slicePtr interface{}, elem interface{}) {
	slicePtrTmp := noEscape(slicePtr)

	if ac.disabled {
		s := reflect.ValueOf(slicePtrTmp).Elem()
		v := reflect.Append(s, reflect.ValueOf(elem))
		s.Set(v)
		return
	}

	slicePtrTp := reflect.TypeOf(slicePtrTmp)
	if slicePtrTp.Kind() != reflect.Ptr || slicePtrTp.Elem().Kind() != reflect.Slice {
		panic("expect pointer to slice")
	}
	sliceElemTp := slicePtrTp.Elem().Elem()

	inputElemTp := reflect.TypeOf(elem)
	if sliceElemTp != inputElemTp && elem != nil {
		panic("elem type not match with slice")
	}

	header := (*reflect.SliceHeader)(data(slicePtrTmp))
	ptr := data(elem)
	if sliceElemTp.Kind() == reflect.Ptr {
		u := uintptr(ptr)
		ptr = unsafe.Pointer(&u)
	}
	ac.sliceAppend(header, ptr, int(sliceElemTp.Size()))
}

func (ac *Allocator) sliceAppend(s *reflect.SliceHeader, elemPtr unsafe.Pointer, elemSz int) {
	// grow
	if s.Len >= s.Cap {
		pre := *s
		s.Cap *= 2
		if s.Cap == 0 {
			s.Cap = 4
		}
		s.Data = uintptr(ac.alloc(s.Cap*elemSz, false))
		memmove(unsafe.Pointer(s.Data), unsafe.Pointer(pre.Data), uintptr(pre.Len*elemSz))
	}

	// append
	if s.Len < s.Cap {
		dst := unsafe.Add(unsafe.Pointer(s.Data), elemSz*s.Len)
		memmove(dst, elemPtr, uintptr(elemSz))
		s.Len++
	}
}

func (ac *Allocator) Enum(e interface{}) interface{} {
	temp := noEscape(e)
	if ac.disabled {
		r := reflect.New(reflect.TypeOf(temp))
		r.Elem().Set(reflect.ValueOf(temp))
		return r.Interface()
	}
	tp := reflect.TypeOf(temp)
	r := ac.typedNew(reflect.PtrTo(tp), 0, false)
	memmove(data(r), data(temp), tp.Size())
	return r
}
