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

package linear_ac

import (
	"fmt"
	"math"
	"reflect"
	"runtime"
	"sync"
	"unsafe"
)

var (
	DbgMode         = false
	DisableLinearAc = false
	ChunkSize       = 1024 * 32
)

// Chunk

type chunk []byte

var chunkPool = syncPool{
	New: func() interface{} {
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
	chunks   []*chunk
	curChunk int
	scanObjs []interface{}
	maps     map[unsafe.Pointer]struct{}
}

// buildInAc switches to native allocator.
var buildInAc = &Allocator{disabled: true}

var acPool = syncPool{
	New: func() interface{} {
		return NewLinearAc()
	},
}

func NewLinearAc() *Allocator {
	ac := &Allocator{
		disabled: DisableLinearAc,
		maps:     map[unsafe.Pointer]struct{}{},
	}
	return ac
}

// Bind allocator to goroutine

var acMap = sync.Map{}

func BindNew() *Allocator {
	ac := acPool.get().(*Allocator)
	acMap.Store(goRoutineId(), ac)
	return ac
}

func Get() *Allocator {
	if val, ok := acMap.Load(goRoutineId()); ok {
		return val.(*Allocator)
	}
	return buildInAc
}

func (ac *Allocator) Unbind() {
	if Get() == ac {
		acMap.Delete(goRoutineId())
	}
}

func (ac *Allocator) Release() {
	if ac == buildInAc {
		return
	}
	ac.Unbind()
	ac.reset()
	acPool.put(ac)
}

func (ac *Allocator) reset() {
	if ac.disabled {
		return
	}

	if DbgMode {
		ac.debugCheck(true)
		ac.scanObjs = ac.scanObjs[:0]
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

	for k := range ac.maps {
		delete(ac.maps, k)
	}

	ac.disabled = DisableLinearAc
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

func (ac *Allocator) typedNew(ptrTp reflect.Type, zero bool) (ret interface{}) {
	objType := ptrTp.Elem()
	ptr := ac.alloc(int(objType.Size()), zero)
	*(*emptyInterface)(unsafe.Pointer(&ret)) = emptyInterface{data(ptrTp), ptr}
	if DbgMode {
		if objType.Kind() == reflect.Struct {
			ac.scanObjs = append(ac.scanObjs, ret)
		}
	}
	return
}

func (ac *Allocator) alloc(need int, zero bool) unsafe.Pointer {
	if len(ac.chunks) == 0 {
		ac.chunks = append(ac.chunks, chunkPool.get().(*chunk))
	}
start:
	cur := ac.chunks[ac.curChunk]
	used := len(*cur)
	if used+need > cap(*cur) {
		if ac.curChunk == len(ac.chunks)-1 {
			var ck *chunk
			if need > ChunkSize {
				c := make(chunk, 0, need)
				ck = &c
			} else {
				ck = chunkPool.get().(*chunk)
			}
			ac.chunks = append(ac.chunks, ck)
		} else if cap(*ac.chunks[ac.curChunk+1]) < need {
			chunkPool.put(ac.chunks[ac.curChunk+1])
			ck := make(chunk, 0, need)
			ac.chunks[ac.curChunk+1] = &ck
		}
		ac.curChunk++
		goto start
	}
	*cur = (*cur)[:used+need]
	ptr := add((*sliceHeader)(unsafe.Pointer(cur)).Data, used)
	if zero {
		clearBytes(ptr, need)
	}
	return ptr
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
	ac.maps[data(m.Interface())] = struct{}{}
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

// CopySlice is useful to create simple slice (simple type as element)
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

// Use 1 instead of nil or MaxUint64 to
// 1. make non-nil check pass.
// 2. generate a recoverable panic.
const trickyAddress = uintptr(1)

func (ac *Allocator) internalPointer(addr uintptr) bool {
	if addr == 0 || addr == trickyAddress {
		return true
	}
	for _, c := range ac.chunks {
		h := (*sliceHeader)(unsafe.Pointer(c))
		if addr >= uintptr(h.Data) && addr < uintptr(h.Data)+uintptr(h.Cap) {
			return true
		}
	}
	return false
}

// NOTE: all memories must be referenced by structs.
func (ac *Allocator) debugCheck(invalidatePointers bool) {
	checked := map[interface{}]struct{}{}
	// reverse order to bypass obfuscated pointers
	for i := len(ac.scanObjs) - 1; i >= 0; i-- {
		ptr := ac.scanObjs[i]
		if _, ok := checked[ptr]; ok {
			continue
		}
		if err := ac.checkRecursively(reflect.ValueOf(ptr), checked, invalidatePointers); err != nil {
			panic(err)
		}
	}
}

// CheckExternalPointers is useful for if you want to check external pointers but don't want to invalidate pointers.
// e.g. using ac as config memory allocator globally.
func (ac *Allocator) CheckExternalPointers() {
	ac.debugCheck(false)
}

func (ac *Allocator) checkRecursively(val reflect.Value, checked map[interface{}]struct{}, invalidatePointers bool) error {
	if val.Kind() == reflect.Ptr {
		if val.Pointer() != trickyAddress && !val.IsNil() {
			if !ac.internalPointer(val.Pointer()) {
				return fmt.Errorf("unexpected external pointer: %+v", val)
			}
			if val.Elem().Type().Kind() == reflect.Struct {
				if err := ac.checkRecursively(val.Elem(), checked, invalidatePointers); err != nil {
					return err
				}
				checked[val.Interface()] = struct{}{}
			}
		}
		return nil
	}

	tp := val.Type()
	fieldName := func(i int) string {
		return fmt.Sprintf("%v.%v", tp.Name(), tp.Field(i).Name)
	}

	if val.Kind() == reflect.Struct {
		for i := 0; i < val.NumField(); i++ {
			f := val.Field(i)

			switch f.Kind() {
			case reflect.Ptr:
				if err := ac.checkRecursively(f, checked, invalidatePointers); err != nil {
					return fmt.Errorf("%v: %v", fieldName(i), err)
				}
				if invalidatePointers {
					*(*uintptr)(unsafe.Pointer(f.UnsafeAddr())) = trickyAddress
				}

			case reflect.Slice:
				h := (*sliceHeader)(unsafe.Pointer(f.UnsafeAddr()))
				if f.Len() > 0 && h.Data != nil {
					if !ac.internalPointer((uintptr)(h.Data)) {
						return fmt.Errorf("%s: unexpected external slice: %s", fieldName(i), f.String())
					}
					for j := 0; j < f.Len(); j++ {
						if err := ac.checkRecursively(f.Index(j), checked, invalidatePointers); err != nil {
							return fmt.Errorf("%v: %v", fieldName(i), err)
						}
					}
				}
				if invalidatePointers {
					h.Data = nil
					h.Len = math.MaxInt32
					h.Cap = math.MaxInt32
				}

			case reflect.Array:
				for j := 0; j < f.Len(); j++ {
					if err := ac.checkRecursively(f.Index(j), checked, invalidatePointers); err != nil {
						return fmt.Errorf("%v: %v", fieldName(i), err)
					}
				}

			case reflect.Map:
				m := *(*unsafe.Pointer)(unsafe.Pointer(f.UnsafeAddr()))
				if _, ok := ac.maps[m]; !ok {
					return fmt.Errorf("%v: unexpected external map: %+v", fieldName(i), f)
				}
				for iter := f.MapRange(); iter.Next(); {
					if err := ac.checkRecursively(iter.Value(), checked, invalidatePointers); err != nil {
						return fmt.Errorf("%v: %v", fieldName(i), err)
					}
				}
			}
		}
	}
	return nil
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
