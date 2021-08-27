//
// Copyright (C) 2020-2021 crazybie@github.com.
//
//
// Linear Allocator
//
// Improve the memory allocation and garbage collection performance.
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

const (
	ChunkSize = 1024 * 2
)

var (
	// DbgCheckPointers checks if user allocates from build-in allocator.
	DbgCheckPointers = true

	DbgDisableLinearAc = false
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
	typ  unsafe.Pointer
	data unsafe.Pointer
}

var BuildInAc = &Allocator{disabled: true}

var (
	uintptrSize = unsafe.Sizeof(uintptr(0))

	boolType   = reflect.TypeOf(true)
	intType    = reflect.TypeOf(0)
	int32Type  = reflect.TypeOf(int32(0))
	uint32Type = reflect.TypeOf(uint32(0))
	int64Type  = reflect.TypeOf(int64(0))
	uint64Type = reflect.TypeOf(uint64(0))
	f32Type    = reflect.TypeOf(float32(0))
	f64Type    = reflect.TypeOf(float64(0))
	strType    = reflect.TypeOf("")
)

type chunk []byte

type Allocator struct {
	disabled      bool
	chunks        []chunk
	curChunk      int
	scanObjs      []interface{}
	knownPointers map[uintptr]struct{}
	maps          map[unsafe.Pointer]struct{}
}

var pool = sync.Pool{
	New: func() interface{} {
		return newLinearAc()
	},
}

func Get() *Allocator {
	return pool.Get().(*Allocator)
}

func (ac *Allocator) Release() {
	if ac == BuildInAc {
		return
	}
	ac.Reset()
	pool.Put(ac)
}

func newLinearAc() *Allocator {
	ac := &Allocator{
		disabled: DbgDisableLinearAc,
		chunks:   []chunk{make(chunk, 0, ChunkSize)},
	}
	if DbgCheckPointers {
		ac.knownPointers = make(map[uintptr]struct{})
	}
	return ac
}

func (ac *Allocator) Reset() {
	if ac.disabled {
		return
	}

	if DbgCheckPointers {
		ac.knownPointers = map[uintptr]struct{}{}
		ac.scanObjs = ac.scanObjs[:0]
	}

	for idx, buf := range ac.chunks {
		ac.chunks[idx] = buf[:0]
	}
	ac.curChunk = 0
	for k := range ac.maps {
		delete(ac.maps, k)
	}
}

func noescape(p interface{}) interface{} {
	var temp interface{}
	r := *(*[2]uintptr)(unsafe.Pointer(&p))
	*(*[2]uintptr)(unsafe.Pointer(&temp)) = r
	return temp
}

func (ac *Allocator) New(ptrToPtr interface{}) {
	tmp := noescape(ptrToPtr)

	if ac.disabled {
		tp := reflect.TypeOf(tmp).Elem().Elem()
		reflect.ValueOf(tmp).Elem().Set(reflect.New(tp))
		return
	}

	tp := reflect.TypeOf(tmp).Elem().Elem()
	v := ac.typedNew(tp, true)
	*(*uintptr)((*emptyInterface)(unsafe.Pointer(&tmp)).data) = (uintptr)((*emptyInterface)(unsafe.Pointer(&v)).data)
}

// New2 is slower than New due to the data copying.
// useful for migration.
func (ac *Allocator) New2(ptr interface{}) (ret interface{}) {
	ptrTemp := noescape(ptr)
	tp := reflect.TypeOf(ptrTemp).Elem()

	if ac.disabled {
		ret = reflect.New(tp).Interface()
	} else {
		ret = ac.typedNew(tp, false)
	}

	copyBytes((*emptyInterface)(unsafe.Pointer(&ptrTemp)).data, (*emptyInterface)(unsafe.Pointer(&ret)).data, int(tp.Size()))
	return
}

func (ac *Allocator) typedNew(tp reflect.Type, zero bool) (ret interface{}) {
	ptr := ac.alloc(int(tp.Size()), zero)
	r := reflect.NewAt(tp, ptr)
	ret = r.Interface()
	if DbgCheckPointers {
		if tp.Kind() == reflect.Struct {
			ac.scanObjs = append(ac.scanObjs, ret)
		}
		ac.knownPointers[uintptr(ptr)] = struct{}{}
	}
	return
}

func (ac *Allocator) alloc(need int, zero bool) unsafe.Pointer {
start:
	cur := &ac.chunks[ac.curChunk]
	used := len(*cur)
	if used+need > cap(*cur) {
		if ac.curChunk == len(ac.chunks)-1 {
			ac.chunks = append(ac.chunks, make(chunk, 0, int32(math.Max(float64(ChunkSize), float64(need)))))
		} else if cap(ac.chunks[ac.curChunk+1]) < need {
			ac.chunks[ac.curChunk+1] = make(chunk, 0, need)
		}
		ac.curChunk++
		goto start
	}
	*cur = (*cur)[:used+need]
	ptr := unsafe.Pointer((uintptr)((*sliceHeader)(unsafe.Pointer(cur)).Data) + uintptr(used))
	if zero {
		clearBytes(ptr, need)
	}
	return ptr
}

func copyBytes(src, dst unsafe.Pointer, len int) {
	alignedEnd := uintptr(len) / uintptrSize * uintptrSize
	i := uintptr(0)
	for ; i < alignedEnd; i += uintptrSize {
		*(*uintptr)(unsafe.Pointer(uintptr(dst) + i)) = *(*uintptr)(unsafe.Pointer(uintptr(src) + i))
	}
	for ; i < uintptr(len); i++ {
		*(*byte)(unsafe.Pointer(uintptr(dst) + i)) = *(*byte)(unsafe.Pointer(uintptr(src) + i))
	}
}

func clearBytes(dst unsafe.Pointer, len int) {
	alignedEnd := uintptr(len) / uintptrSize * uintptrSize
	i := uintptr(0)
	for ; i < alignedEnd; i += uintptrSize {
		*(*uintptr)(unsafe.Pointer(uintptr(dst) + i)) = 0
	}
	for ; i < uintptr(len); i++ {
		*(*byte)(unsafe.Pointer(uintptr(dst) + i)) = 0
	}
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
	var mapPtrTemp = noescape(mapPtr)

	if ac.disabled {
		tp := reflect.TypeOf(mapPtrTemp).Elem()
		reflect.ValueOf(mapPtrTemp).Elem().Set(reflect.MakeMap(tp))
		return
	}

	m := reflect.MakeMap(reflect.TypeOf(mapPtrTemp).Elem())
	i := m.Interface()
	v := (*emptyInterface)(unsafe.Pointer(&i))
	reflect.ValueOf(mapPtrTemp).Elem().Set(m)

	if ac.maps == nil {
		ac.maps = make(map[unsafe.Pointer]struct{})
	}
	ac.maps[v.data] = struct{}{}

	runtime.KeepAlive(mapPtrTemp)
}

func (ac *Allocator) NewSlice(slicePtr interface{}, len, cap_ int) {
	var slicePtrTmp = noescape(slicePtr)

	if ac.disabled {
		v := reflect.MakeSlice(reflect.TypeOf(slicePtrTmp).Elem(), len, cap_)
		reflect.ValueOf(slicePtrTmp).Elem().Set(v)
		return
	}

	refSlicePtrType := reflect.TypeOf(slicePtrTmp)
	if refSlicePtrType.Kind() != reflect.Ptr || refSlicePtrType.Elem().Kind() != reflect.Slice {
		panic(fmt.Errorf("need a pointer to slice"))
	}

	if cap_ < len {
		cap_ = len
	}

	sliceEface := (*emptyInterface)(unsafe.Pointer(&slicePtrTmp))
	slice_ := (*sliceHeader)(sliceEface.data)
	slice_.Data = ac.alloc(cap_*int(refSlicePtrType.Elem().Elem().Size()), false)
	slice_.Len = len
	slice_.Cap = cap_
	if DbgCheckPointers {
		ac.knownPointers[uintptr(slice_.Data)] = struct{}{}
	}
}

// SliceAppend append pointers to slice
func (ac *Allocator) SliceAppend(slicePtr interface{}, itemPtr interface{}) {
	var slicePtrTmp = noescape(slicePtr)

	if ac.disabled {
		s := reflect.ValueOf(slicePtrTmp).Elem()
		v := reflect.Append(s, reflect.ValueOf(itemPtr))
		s.Set(v)
		return
	}

	refSlicePtrTp := reflect.TypeOf(slicePtrTmp)
	if refSlicePtrTp.Kind() != reflect.Ptr || refSlicePtrTp.Elem().Kind() != reflect.Slice {
		panic(fmt.Errorf("expect pointer to slice"))
	}
	refItemPtrTp := reflect.TypeOf(itemPtr)
	if refSlicePtrTp.Elem().Elem() != refItemPtrTp {
		panic(fmt.Errorf("elem type not match with slice"))
	}

	sliceEface := (*emptyInterface)(unsafe.Pointer(&slicePtrTmp))
	slice_ := (*sliceHeader)(sliceEface.data)
	itemEface := (*emptyInterface)(unsafe.Pointer(&itemPtr))
	elemSz := int(refItemPtrTp.Size())

	// grow
	if slice_.Len >= slice_.Cap {
		pre := *slice_
		if slice_.Cap >= 16 {
			slice_.Cap = int(float32(slice_.Cap) * 1.5)
		} else {
			slice_.Cap *= 2
		}
		if slice_.Cap == 0 {
			slice_.Cap = 1
		}
		slice_.Data = ac.alloc(slice_.Cap*elemSz, false)
		copyBytes(pre.Data, slice_.Data, pre.Len*elemSz)
		slice_.Len = pre.Len

		delete(ac.knownPointers, uintptr(pre.Data))
		if DbgCheckPointers {
			ac.knownPointers[uintptr(slice_.Data)] = struct{}{}
		}
	}

	// append
	if slice_.Len < slice_.Cap {
		d := unsafe.Pointer(uintptr(slice_.Data) + uintptr(elemSz)*uintptr(slice_.Len))
		if refItemPtrTp.Kind() == reflect.Ptr {
			*(*uintptr)(d) = (uintptr)(itemEface.data)
		} else {
			copyBytes(itemEface.data, d, elemSz)
		}
		slice_.Len++
	}
}

func (ac *Allocator) Enum(e interface{}) interface{} {
	var temp = noescape(e)
	if ac.disabled {
		r := reflect.New(reflect.TypeOf(temp))
		r.Elem().Set(reflect.ValueOf(temp))
		return r.Interface()
	}
	r := ac.typedNew(reflect.TypeOf(temp), false)
	*((*[2]*uintptr)(unsafe.Pointer(&r)))[1] = *(*uintptr)((*emptyInterface)(unsafe.Pointer(&temp)).data)
	return r
}

func (ac *Allocator) CheckPointers() {
	if ac.disabled {
		return
	}
	for _, ptr := range ac.scanObjs {
		if err := ac.checkRecursively(reflect.ValueOf(ptr)); err != nil {
			panic(err)
		}
	}
}

func (ac *Allocator) checkRecursively(pe reflect.Value) error {
	if pe.Kind() == reflect.Ptr {
		if !pe.IsNil() {
			if _, ok := ac.knownPointers[pe.Pointer()]; !ok {
				return fmt.Errorf("unexpected external pointer: %+v", pe)
			}
			if pe.Elem().Type().Kind() == reflect.Struct {
				return ac.checkRecursively(pe.Elem())
			}
		}
		return nil
	}
	fieldName := func(i int) string {
		return fmt.Sprintf("%v.%v", pe.Type().Name(), pe.Type().Field(i).Name)
	}
	if pe.Kind() == reflect.Struct {
		for i := 0; i < pe.NumField(); i++ {
			f := pe.Field(i)
			switch f.Kind() {
			case reflect.Ptr:
				if err := ac.checkRecursively(f); err != nil {
					return fmt.Errorf("%v: %v", fieldName(i), err)
				}
			case reflect.Slice:
				if f.Len() > 0 {
					dataPtr := (uintptr)((*sliceHeader)(unsafe.Pointer(f.UnsafeAddr())).Data)
					if _, ok := ac.knownPointers[dataPtr]; !ok {
						return fmt.Errorf("%s: unexpected external pointer: %s", fieldName(i), f.String())
					}
				}
				fallthrough
			case reflect.Array:
				for j := 0; j < f.Len(); j++ {
					if err := ac.checkRecursively(f.Index(j)); err != nil {
						return fmt.Errorf("%v: %v", fieldName(i), err)
					}
				}
			case reflect.Map:
				m := *(*unsafe.Pointer)(unsafe.Pointer(f.UnsafeAddr()))
				if _, ok := ac.maps[m]; !ok {
					return fmt.Errorf("%v: unexpected external pointer: %+v", fieldName(i), f)
				}
				for iter := f.MapRange(); iter.Next(); {
					if err := ac.checkRecursively(iter.Value()); err != nil {
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
		r = ac.typedNew(boolType, false).(*bool)
	}
	*r = v
	return
}

func (ac *Allocator) Int(v int) (r *int) {
	if ac.disabled {
		r = new(int)
	} else {
		r = ac.typedNew(intType, false).(*int)
	}
	*r = v
	return
}

func (ac *Allocator) Int32(v int32) (r *int32) {
	if ac.disabled {
		r = new(int32)
	} else {
		r = ac.typedNew(int32Type, false).(*int32)
	}
	*r = v
	return
}

func (ac *Allocator) Uint32(v uint32) (r *uint32) {
	if ac.disabled {
		r = new(uint32)
	} else {
		r = ac.typedNew(uint32Type, false).(*uint32)
	}
	*r = v
	return
}

func (ac *Allocator) Int64(v int64) (r *int64) {
	if ac.disabled {
		r = new(int64)
	} else {
		r = ac.typedNew(int64Type, false).(*int64)
	}
	*r = v
	return
}

func (ac *Allocator) Uint64(v uint64) (r *uint64) {
	if ac.disabled {
		r = new(uint64)
	} else {
		r = ac.typedNew(uint64Type, false).(*uint64)
	}
	*r = v
	return
}

func (ac *Allocator) Float32(v float32) (r *float32) {
	if ac.disabled {
		r = new(float32)
	} else {
		r = ac.typedNew(f32Type, false).(*float32)
	}
	*r = v
	return
}

func (ac *Allocator) Float64(v float64) (r *float64) {
	if ac.disabled {
		r = new(float64)
	} else {
		r = ac.typedNew(f64Type, false).(*float64)
	}
	*r = v
	return
}

func (ac *Allocator) String(v string) (r *string) {
	if ac.disabled {
		r = new(string)
		*r = v
	} else {
		r = ac.typedNew(strType, false).(*string)
		*r = ac.NewString(v)
	}
	return
}
