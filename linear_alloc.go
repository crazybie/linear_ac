//
// Linear Allocator
//
// # Goal
// Speed up the memory allocation and garbage collection performance.
//
// # Possible Usage
// 1. global memory never need to be released. (configs, global systems)
// 2. temporary objects with deterministic lifetime. (buffers send to network)
//
// # TODO:
// 1. SliceAppend support value type as elem
//

package linear_ac

import (
	"fmt"
	"math"
	"reflect"
	"runtime"
	"sync/atomic"
	"unsafe"
)

///////////////////////////////////////////////////////////////////
// WARNING:
// The following structs must be matched with
// the version from the current golang runtime.
///////////////////////////////////////////////////////////////////

/// MatchWithGolangRuntime Start

type sliceHeader struct {
	Data unsafe.Pointer
	Len  int
	Cap  int
}

type stringHeader struct {
	Data unsafe.Pointer
	Len  int
}

type rtype struct {
	size       uintptr
	ptrdata    uintptr
	hash       uint32
	tflag      uint8
	align      uint8
	fieldAlign uint8
	kind       uint8
	equal      func(unsafe.Pointer, unsafe.Pointer) bool
	gcdata     *byte
	str        int32
	ptrToThis  int32
}

const rtypeSize = unsafe.Sizeof(rtype{})

type emptyInterface struct {
	typ  *rtype
	data unsafe.Pointer
}

type sliceType struct {
	rtype
	elem *rtype
}

type ptrType struct {
	rtype
	elem *rtype
}

/// MatchWithGolangRuntime End

var (
	// DbgCheckPointers checks if user allocates from build-in allocator.
	DbgCheckPointers int32 = 0
)

const (
	BlockSize = 1024 * 8
)

var (
	uintptrSize = unsafe.Sizeof(uintptr(0))

	boolType  = reflect.TypeOf(true)
	intType   = reflect.TypeOf(0)
	int32Type = reflect.TypeOf(int32(0))
	int64Type = reflect.TypeOf(int64(0))
	f32Type   = reflect.TypeOf(float32(0))
	f64Type   = reflect.TypeOf(float64(0))
	strType   = reflect.TypeOf("")

	reflectTypeSize = reflect.TypeOf(intType).Elem().Size()
)

type block []byte

type LinearAllocator struct {
	staticBlock   [1024]byte
	blocks        []block
	curBlock      int
	scanObjs      []reflect.Value
	knownPointers map[uintptr]interface{}
	maps          map[unsafe.Pointer]struct{}
}

func NewLinearAllocator() (ret *LinearAllocator) {
	ret = &LinearAllocator{}
	ret.blocks = append(ret.blocks, ret.staticBlock[:0])

	if atomic.LoadInt32(&DbgCheckPointers) == 1 {
		ret.knownPointers = make(map[uintptr]interface{})
	}
	if reflectTypeSize != rtypeSize {
		panic(fmt.Errorf("golang runtime structs mismatch"))
	}
	return
}

func (ac *LinearAllocator) Reset() {
	if atomic.LoadInt32(&DbgCheckPointers) == 1 {
		ac.CheckPointers()
		for k := range ac.knownPointers {
			delete(ac.knownPointers, k)
		}
		ac.scanObjs = ac.scanObjs[:0]
	}

	for idx, buf := range ac.blocks {
		ac.blocks[idx] = buf[:0]
	}
	ac.curBlock = 0
	for k := range ac.maps {
		delete(ac.maps, k)
	}
}

func (ac *LinearAllocator) New(ptrToPtr interface{}) {
	var ptrToPtrTemp interface{}
	// store in an uintptr to cheat the escape analyser
	p := *(*[2]uintptr)(unsafe.Pointer(&ptrToPtr))
	*(*[2]uintptr)(unsafe.Pointer(&ptrToPtrTemp)) = p

	tp := reflect.TypeOf(ptrToPtrTemp).Elem().Elem()
	v := ac.TypedNew(tp)
	srcEface := (*emptyInterface)(unsafe.Pointer(&v))
	*(*uintptr)(unsafe.Pointer(p[1])) = (uintptr)(srcEface.data)

	runtime.KeepAlive(ptrToPtr)
}

// New2 is slower than New due to the data copying.
func (ac *LinearAllocator) New2(ptr interface{}) interface{} {
	var ptrTemp interface{}
	// store in an uintptr to cheat the escape analyser
	p := *(*[2]uintptr)(unsafe.Pointer(&ptr))
	*(*[2]uintptr)(unsafe.Pointer(&ptrTemp)) = p

	tp := reflect.TypeOf(ptrTemp).Elem()
	ret := ac.TypedNew(tp)
	copyBytes((*emptyInterface)(unsafe.Pointer(&ptrTemp)).data, (*emptyInterface)(unsafe.Pointer(&ret)).data, int(tp.Size()))

	runtime.KeepAlive(ptr)
	return ret
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

func (ac *LinearAllocator) alloc(need int) unsafe.Pointer {
start:
	buf := &ac.blocks[ac.curBlock]
	used := len(*buf)
	if used+need > cap(*buf) {
		if ac.curBlock == len(ac.blocks)-1 {
			ac.blocks = append(ac.blocks, make(block, 0, int32(math.Max(float64(BlockSize), float64(need)))))
		} else if cap(ac.blocks[ac.curBlock+1]) < need {
			ac.blocks[ac.curBlock+1] = make(block, 0, need)
		}
		ac.curBlock++
		goto start
	}
	*buf = (*buf)[:used+need]
	ptr := unsafe.Pointer(&(*buf)[used])
	clearBytes(ptr, need)
	return ptr
}

func (ac *LinearAllocator) TypedNew(tp reflect.Type) (ret interface{}) {
	ptr := ac.alloc(int(tp.Size()))
	r := reflect.NewAt(tp, ptr)
	ret = r.Interface()
	if atomic.LoadInt32(&DbgCheckPointers) == 1 {
		if tp.Kind() == reflect.Struct {
			ac.scanObjs = append(ac.scanObjs, r)
		}
		ac.knownPointers[uintptr(ptr)] = ret
	}
	return
}

func (ac *LinearAllocator) NewString(v string) string {
	h := (*stringHeader)(unsafe.Pointer(&v))
	ptr := ac.alloc(h.Len)
	copyBytes(h.Data, ptr, h.Len)
	h.Data = ptr
	return v
}

// NewMap use build-in allocator
func (ac *LinearAllocator) NewMap(mapPtr interface{}) {
	var mapPtrTemp interface{}
	// store in an uintptr to cheat the escape analyser
	p := *(*[2]uintptr)(unsafe.Pointer(&mapPtr))
	*(*[2]uintptr)(unsafe.Pointer(&mapPtrTemp)) = p

	m := reflect.MakeMap(reflect.TypeOf(mapPtrTemp).Elem())
	i := m.Interface()
	v := (*emptyInterface)(unsafe.Pointer(&i))
	reflect.ValueOf(mapPtrTemp).Elem().Set(m)

	if ac.maps == nil {
		ac.maps = make(map[unsafe.Pointer]struct{})
	}
	ac.maps[v.data] = struct{}{}

	runtime.KeepAlive(mapPtr)
}

func (ac *LinearAllocator) NewSlice(slicePtr interface{}, cap_ int) {
	var slicePtrTmp interface{}
	// store in an uintptr to cheat the escape analyser
	p := *(*[2]uintptr)(unsafe.Pointer(&slicePtr))
	*(*[2]uintptr)(unsafe.Pointer(&slicePtrTmp)) = p

	refSlicePtrType := reflect.TypeOf(slicePtrTmp)
	if refSlicePtrType.Kind() != reflect.Ptr || refSlicePtrType.Elem().Kind() != reflect.Slice {
		panic(fmt.Errorf("need a pointer to slice"))
	}

	sliceEface := (*emptyInterface)(unsafe.Pointer(&slicePtrTmp))
	slice_ := (*sliceHeader)(sliceEface.data)
	ptrTyp := (*ptrType)(unsafe.Pointer(sliceEface.typ))
	sliceTyp := (*sliceType)(unsafe.Pointer(ptrTyp.elem))
	slice_.Data = ac.alloc(cap_ * int(sliceTyp.elem.size))
	slice_.Len = 0
	slice_.Cap = cap_
}

// SliceAppend append pointers to slice
func (ac *LinearAllocator) SliceAppend(slicePtr interface{}, itemPtr interface{}) {
	var slicePtrTmp interface{}
	// store in an uintptr to cheat the escape analyser
	p := *(*[2]uintptr)(unsafe.Pointer(&slicePtr))
	*(*[2]uintptr)(unsafe.Pointer(&slicePtrTmp)) = p

	refSlicePtrTp := reflect.TypeOf(slicePtrTmp)
	if refSlicePtrTp.Kind() != reflect.Ptr || refSlicePtrTp.Elem().Kind() != reflect.Slice {
		panic(fmt.Errorf("expect pointer to slice"))
	}
	refItemPtrTp := reflect.TypeOf(itemPtr)
	if refItemPtrTp.Kind() != reflect.Ptr || refItemPtrTp.Elem().Kind() != reflect.Struct {
		panic(fmt.Errorf("expect pointer to struct"))
	}
	if refSlicePtrTp.Elem().Elem() != refItemPtrTp {
		panic(fmt.Errorf("elem type not match with slice"))
	}

	sliceEface := (*emptyInterface)(unsafe.Pointer(&slicePtrTmp))
	slice_ := (*sliceHeader)(sliceEface.data)
	ptrTyp := (*ptrType)(unsafe.Pointer(sliceEface.typ))
	sliceTyp := (*sliceType)(unsafe.Pointer(ptrTyp.elem))
	itemEface := (*emptyInterface)(unsafe.Pointer(&itemPtr))
	elemSz := int(sliceTyp.elem.size)
	pointerChecking := atomic.LoadInt32(&DbgCheckPointers) == 1

	if elemSz > int(unsafe.Sizeof(uintptr(0))) {
		panic(fmt.Errorf("unsupported slice"))
	}

	// grow
	if slice_.Len >= slice_.Cap {
		pre := *slice_
		slice_.Cap = slice_.Cap * 2
		if slice_.Cap == 0 {
			slice_.Cap = 1
		}
		slice_.Data = ac.alloc(slice_.Cap * elemSz)
		copyBytes(pre.Data, slice_.Data, pre.Len*elemSz)
		slice_.Len = pre.Len

		if pointerChecking {
			delete(ac.knownPointers, uintptr(pre.Data))
		}
	}

	// append
	if slice_.Len < slice_.Cap {
		d := unsafe.Pointer(uintptr(slice_.Data) + sliceTyp.elem.size*uintptr(slice_.Len))
		*(*uintptr)(d) = (uintptr)(itemEface.data)
		slice_.Len++

		if pointerChecking {
			ac.knownPointers[uintptr(slice_.Data)] = slicePtrTmp
		}
	}

	runtime.KeepAlive(slicePtr)
}

func (ac *LinearAllocator) CheckPointers() {
	for _, ptr := range ac.scanObjs {
		if err := ac.checkRecursively(ptr); err != nil {
			panic(err)
		}
	}
}

func (ac *LinearAllocator) checkRecursively(pe reflect.Value) error {
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
				if _, ok := ac.knownPointers[f.Index(0).UnsafeAddr()]; !ok {
					return fmt.Errorf("%v: unexpected external pointer: %+v", fieldName(i), f)
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

func (ac *LinearAllocator) Bool(v bool) (r *bool) {
	r = ac.TypedNew(boolType).(*bool)
	*r = v
	return
}

func (ac *LinearAllocator) Int(v int) (r *int) {
	r = ac.TypedNew(intType).(*int)
	*r = v
	return
}

func (ac *LinearAllocator) Int32(v int32) (r *int32) {
	r = ac.TypedNew(int32Type).(*int32)
	*r = v
	return
}

func (ac *LinearAllocator) Int64(v int64) (r *int64) {
	r = ac.TypedNew(int64Type).(*int64)
	*r = v
	return
}

func (ac *LinearAllocator) Float32(v float32) (r *float32) {
	r = ac.TypedNew(f32Type).(*float32)
	*r = v
	return
}

func (ac *LinearAllocator) Float64(v float64) (r *float64) {
	r = ac.TypedNew(f64Type).(*float64)
	*r = v
	return
}

func (ac *LinearAllocator) String(v string) (r *string) {
	r = ac.TypedNew(strType).(*string)
	*r = ac.NewString(v)
	return
}
