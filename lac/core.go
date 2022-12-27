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

const PtrSize = int(unsafe.Sizeof(uintptr(0)))

var (
	DbgMode         = false
	DisableLinearAc = false
	ChunkSize       = 1024 * 32
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
	disabled bool
	sync.Mutex
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
		ac.externalSlice = append(ac.externalSlice, (*sliceHeader)(d).Data)
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
			chunkPool.put(ck)
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

func (ac *Allocator) typedNew(ptrTp reflect.Type, zero bool) (ret interface{}) {
	objType := ptrTp.Elem()
	ptr := ac.alloc(int(objType.Size()), zero)
	*(*emptyInterface)(unsafe.Pointer(&ret)) = emptyInterface{data(ptrTp), ptr}
	if DbgMode {
		if objType.Kind() == reflect.Struct {
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
			var ck *chunk
			if aligned > ChunkSize {
				c := make(chunk, 0, aligned)
				ck = &c
			} else {
				ck = chunkPool.get()
			}
			ac.chunks = append(ac.chunks, ck)
		} else if cap(*ac.chunks[ac.curChunk+1]) < aligned {
			chunkPool.put(ac.chunks[ac.curChunk+1])
			ck := make(chunk, 0, aligned)
			ac.chunks[ac.curChunk+1] = &ck
		}
		ac.curChunk++
		goto start
	}
	*cur = (*cur)[:used+aligned]
	ptr := add((*sliceHeader)(unsafe.Pointer(cur)).Data, used)
	if zero {
		clearBytes(ptr, aligned)
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
	v := ac.typedNew(tp, true)
	reflect.ValueOf(tmp).Elem().Set(reflect.ValueOf(v))
}

// NewCopy is useful for code migration.
// native mode is slower than new() due to the additional memory move from stack to heap,
// this is on purpose to avoid heap alloc in linear mode.
func (ac *Allocator) NewCopy(ptr interface{}) (ret interface{}) {
	ptrTemp := noEscape(ptr)
	ptrType := reflect.TypeOf(ptrTemp)
	tp := ptrType.Elem()

	if ac.disabled {
		ret = reflect.New(tp).Interface()
		reflect_typedmemmove(data(tp), data(ret), data(ptrTemp))
	} else {
		ret = ac.typedNew(ptrType, false)
		copyBytes(data(ptrTemp), data(ret), int(tp.Size()))
	}
	return
}

func (ac *Allocator) NewString(v string) string {
	if ac.disabled {
		return v
	}
	h := (*stringHeader)(unsafe.Pointer(&v))
	ptr := ac.alloc(h.Len, false)
	copyBytes(h.Data, ptr, h.Len)
	h.Data = ptr
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

	slice := (*sliceHeader)(data(slicePtrTmp))
	if cap < len {
		cap = len
	}
	slice.Data = ac.alloc(cap*int(slicePtrType.Elem().Elem().Size()), false)
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
	header := (*sliceHeader)(data(sliceTmp))
	size := int(elemType.Size()) * header.Len
	dst := ac.alloc(size, false)
	copyBytes(header.Data, dst, size)
	header.Data = dst

	runtime.KeepAlive(slice)
	return ret
}

// SliceAppend with no malloc.
// NOTE: the generic version has weird malloc thus not provided.
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
	inputElemTp := reflect.TypeOf(elem)
	sliceElemTp := slicePtrTp.Elem().Elem()
	if sliceElemTp != inputElemTp && elem != nil {
		panic("elem type not match with slice")
	}

	header := (*sliceHeader)(data(slicePtrTmp))
	elemSz := int(sliceElemTp.Size())

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
		elemData := data(elem)
		dst := add(header.Data, elemSz*header.Len)
		if sliceElemTp.Kind() == reflect.Ptr {
			*(*unsafe.Pointer)(dst) = elemData
		} else {
			copyBytes(elemData, dst, elemSz)
		}
		header.Len++
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
	r := ac.typedNew(reflect.PtrTo(tp), false)
	copyBytes(data(temp), data(r), int(tp.Size()))
	return r
}
