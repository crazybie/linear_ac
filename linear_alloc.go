package linear_ac

import (
	"fmt"
	"math"
	"reflect"
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

// DbgCheckPointers checks if user allocates from build-in allocator.
var DbgCheckPointers int32 = 1

type buffer []byte

type LinearAllocator struct {
	bufferAllocSize       int
	buffers               []buffer
	curBuffer             int
	scanObjs              []reflect.Value
	knownPtrs             map[uintptr]interface{}
	enablePointerChecking bool
}

func NewLinearAllocator() (ret *LinearAllocator) {
	defaultCap := 1024 * 4
	ret = &LinearAllocator{
		bufferAllocSize:       defaultCap,
		buffers:               []buffer{make(buffer, 0, defaultCap)},
		curBuffer:             0,
		knownPtrs:             make(map[uintptr]interface{}),
		enablePointerChecking: atomic.LoadInt32(&DbgCheckPointers) == 1,
	}
	return
}

func (ac *LinearAllocator) FreeAll() {
	if ac.enablePointerChecking {
		ac.checkPointers()
		for k := range ac.knownPtrs {
			delete(ac.knownPtrs, k)
		}
		ac.scanObjs = ac.scanObjs[:0]
	}

	for idx, buf := range ac.buffers {
		ac.buffers[idx] = buf[:0]
	}
	ac.curBuffer = 0
}

func (ac *LinearAllocator) New(ptrToPtr interface{}) {
	tp := reflect.TypeOf(ptrToPtr).Elem().Elem()
	reflect.ValueOf(ptrToPtr).Elem().Set(reflect.ValueOf(ac.typedAlloc(tp)))
}

func copyBytes(src, dst unsafe.Pointer, len int) {
	align := unsafe.Sizeof(uintptr(0))
	alignedEnd := uintptr(len) / align * align
	i := uintptr(0)
	for ; i < alignedEnd; i += align {
		*(*uintptr)(unsafe.Pointer(uintptr(dst) + i)) = *(*uintptr)(unsafe.Pointer(uintptr(src) + i))
	}
	for ; i < uintptr(len); i++ {
		*(*byte)(unsafe.Pointer(uintptr(dst) + i)) = *(*byte)(unsafe.Pointer(uintptr(src) + i))
	}
}

func (ac *LinearAllocator) alloc(need int) unsafe.Pointer {
start:
	buf := &ac.buffers[ac.curBuffer]
	used := len(*buf)
	if used+need > cap(*buf) {
		ac.buffers = append(ac.buffers, make(buffer, 0, int32(math.Max(float64(ac.bufferAllocSize), float64(need)))))
		ac.curBuffer++
		goto start
	}
	*buf = (*buf)[:used+need]
	for i := 0; i < need; i++ {
		(*buf)[i+used] = 0
	}
	return unsafe.Pointer(&(*buf)[used])
}

func (ac *LinearAllocator) typedAlloc(tp reflect.Type) (ret interface{}) {
	ptr := ac.alloc(int(tp.Size()))
	r := reflect.NewAt(tp, ptr)
	ret = r.Interface()
	if ac.enablePointerChecking {
		if tp.Kind() == reflect.Struct {
			ac.scanObjs = append(ac.scanObjs, r)
		}
		ac.knownPtrs[uintptr(ptr)] = ret
	}
	return
}

// SliceAppend append pointers to slice
// TODO: support value type as slice elem
func (ac *LinearAllocator) SliceAppend(slicePtr interface{}, itemPtr interface{}) {
	refSlicePtrTp := reflect.TypeOf(slicePtr)
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

	sliceEface := (*emptyInterface)(unsafe.Pointer(&slicePtr))
	slice_ := (*sliceHeader)(sliceEface.data)
	ptrTyp := (*ptrType)(unsafe.Pointer(sliceEface.typ))
	sliceTyp := (*sliceType)(unsafe.Pointer(ptrTyp.elem))
	itemEface := (*emptyInterface)(unsafe.Pointer(&itemPtr))
	elemSz := int(sliceTyp.elem.size)

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

		if ac.enablePointerChecking {
			delete(ac.knownPtrs, uintptr(pre.Data))
		}
	}

	// append
	if slice_.Len < slice_.Cap {
		cur := uintptr(slice_.Data) + sliceTyp.elem.size*uintptr(slice_.Len)
		*(*uintptr)(unsafe.Pointer(cur)) = (uintptr)(itemEface.data)
		slice_.Len++

		if ac.enablePointerChecking {
			ac.knownPtrs[uintptr(slice_.Data)] = slicePtr
		}
	}
}

func (ac *LinearAllocator) NewString(v string) string {
	h := (*stringHeader)(unsafe.Pointer(&v))
	ptr := ac.alloc(h.Len)
	copyBytes(h.Data, ptr, h.Len)
	h.Data = ptr
	return v
}

func (ac *LinearAllocator) checkPointers() {
	for _, ptr := range ac.scanObjs {
		if err := ac.checkRecursively(ptr); err != nil {
			panic(err)
		}
	}
}

func (ac *LinearAllocator) checkRecursively(pe reflect.Value) error {
	if pe.Kind() == reflect.Ptr {
		if !pe.IsNil() {
			if _, ok := ac.knownPtrs[pe.Pointer()]; !ok {
				return fmt.Errorf("unexpected external pointer: %+v", pe)
			}
			if pe.Elem().Type().Kind() == reflect.Struct {
				return ac.checkRecursively(pe.Elem())
			}
		}
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
				if _, ok := ac.knownPtrs[f.Index(0).UnsafeAddr()]; !ok {
					return fmt.Errorf("%v: unexpected external pointer: %+v", fieldName(i), f)
				}
				fallthrough
			case reflect.Array:
				for j := 0; j < f.Len(); j++ {
					if err := ac.checkRecursively(f.Index(j)); err != nil {
						return fmt.Errorf("%v: %v", fieldName(i), err)
					}
				}
			}
		}
	}
	return nil
}

var boolType = reflect.TypeOf(true)
var intType = reflect.TypeOf(int(0))
var int32Type = reflect.TypeOf(int32(0))
var int64Type = reflect.TypeOf(int64(0))
var f32Type = reflect.TypeOf(float32(0))
var f64Type = reflect.TypeOf(float64(0))
var strType = reflect.TypeOf("")

func (ac *LinearAllocator) Bool(v bool) (r *bool) {
	r = ac.typedAlloc(boolType).(*bool)
	*r = v
	return
}

func (ac *LinearAllocator) Int(v int) (r *int) {
	r = ac.typedAlloc(intType).(*int)
	*r = v
	return
}

func (ac *LinearAllocator) Int32(v int32) (r *int32) {
	r = ac.typedAlloc(int32Type).(*int32)
	*r = v
	return
}

func (ac *LinearAllocator) Int64(v int64) (r *int64) {
	r = ac.typedAlloc(int64Type).(*int64)
	*r = v
	return
}

func (ac *LinearAllocator) Float32(v float32) (r *float32) {
	r = ac.typedAlloc(f32Type).(*float32)
	*r = v
	return
}

func (ac *LinearAllocator) Float64(v float64) (r *float64) {
	r = ac.typedAlloc(f64Type).(*float64)
	*r = v
	return
}

func (ac *LinearAllocator) String(v string) (r *string) {
	r = ac.typedAlloc(strType).(*string)
	*r = ac.NewString(v)
	return
}
