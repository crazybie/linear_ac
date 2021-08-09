package auto_pb

import (
	"fmt"
	"reflect"
	"sync/atomic"
	"unsafe"
)

var checkPointers int32 = 1

type LinearAllocator struct {
	buffer   []byte
	scanObjs []reflect.Value
	// prevent gc from collecting the objects referenced by pointers in buffer.
	knownPtrs map[uintptr]interface{}
	Miss      int
}

func NewLinearAllocator(cap int) *LinearAllocator {
	return &LinearAllocator{
		buffer:    make([]byte, 0, cap),
		scanObjs:  make([]reflect.Value, 0, cap/8/4),
		knownPtrs: make(map[uintptr]interface{}, cap/32),
	}
}

func (ac *LinearAllocator) FreeAll() {
	if atomic.LoadInt32(&checkPointers) == 1 {
		ac.checkPointers()
	}

	ac.buffer = ac.buffer[:0]
	ac.scanObjs = ac.scanObjs[:0]
	for k := range ac.knownPtrs {
		delete(ac.knownPtrs, k)
	}
}

func (ac *LinearAllocator) New(ptrToPtr interface{}) {
	tp := reflect.TypeOf(ptrToPtr).Elem().Elem()
	reflect.ValueOf(ptrToPtr).Elem().Set(reflect.ValueOf(ac.Alloc(tp)))
}

func (ac *LinearAllocator) Alloc(tp reflect.Type) interface{} {
	used, need := len(ac.buffer), int(tp.Size())
	if used+need > cap(ac.buffer) {
		ac.Miss++
		r := reflect.New(tp)
		ac.knownPtrs[r.Elem().UnsafeAddr()] = r.Interface()
		return r.Interface()
	}

	ac.buffer = ac.buffer[:used+need]
	ptr := unsafe.Pointer(&ac.buffer[used])
	for i := 0; i < need; i++ {
		ac.buffer[i+used] = 0
	}

	r := reflect.NewAt(tp, ptr)
	if tp.Kind() == reflect.Struct {
		ac.scanObjs = append(ac.scanObjs, r)
	}
	ac.knownPtrs[uintptr(ptr)] = r.Interface()
	return r.Interface()
}

func (ac *LinearAllocator) checkPointers() {
	for _, ptr := range ac.scanObjs {
		if err := ac.check(ptr); err != nil {
			panic(err)
		}
	}
}

func (ac *LinearAllocator) check(pe reflect.Value) error {
	if pe.Kind() == reflect.Ptr {
		if !pe.IsNil() {
			if _, ok := ac.knownPtrs[pe.Pointer()]; !ok {
				return fmt.Errorf("unexpected external pointer: %+v", pe)
			}
			if pe.Elem().Type().Kind() == reflect.Struct {
				return ac.check(pe.Elem())
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
				if err := ac.check(f); err != nil {
					return fmt.Errorf("%v: %v", fieldName(i), err)
				}
			case reflect.Slice:
				if _, ok := ac.knownPtrs[f.Index(0).UnsafeAddr()]; !ok {
					return fmt.Errorf("%v: unexpected external pointer: %+v", fieldName(i), f)
				}
				fallthrough
			case reflect.Array:
				for j := 0; j < f.Len(); j++ {
					if err := ac.check(f.Index(j)); err != nil {
						return fmt.Errorf("%v: %v", fieldName(i), err)
					}
				}
			}
		}
	}
	return nil
}

func (ac *LinearAllocator) Append(slicePtr interface{}, elem interface{}) {
	slicePtrVal := reflect.ValueOf(slicePtr)
	sliceVal := slicePtrVal.Elem()
	newSlice := reflect.Append(sliceVal, reflect.ValueOf(elem))
	sliceVal.Set(newSlice)
	if sliceVal.Len() > 0 {
		delete(ac.knownPtrs, sliceVal.Index(0).UnsafeAddr())
	}
	ac.knownPtrs[newSlice.Index(0).UnsafeAddr()] = newSlice.Interface()
}

func (ac *LinearAllocator) Bool(v bool) (r *bool) {
	r = ac.Alloc(reflect.TypeOf(v)).(*bool)
	*r = v
	return
}

func (ac *LinearAllocator) Int(v int) (r *int) {
	r = ac.Alloc(reflect.TypeOf(v)).(*int)
	*r = v
	return
}

func (ac *LinearAllocator) Int32(v int32) (r *int32) {
	r = ac.Alloc(reflect.TypeOf(v)).(*int32)
	*r = v
	return
}
func (ac *LinearAllocator) Int64(v int64) (r *int64) {
	r = ac.Alloc(reflect.TypeOf(v)).(*int64)
	*r = v
	return
}

func (ac *LinearAllocator) Float32(v float32) (r *float32) {
	r = ac.Alloc(reflect.TypeOf(v)).(*float32)
	*r = v
	return
}

func (ac *LinearAllocator) Float64(v float64) (r *float64) {
	r = ac.Alloc(reflect.TypeOf(v)).(*float64)
	*r = v
	return
}

func (ac *LinearAllocator) String(v string) (r *string) {
	r = ac.Alloc(reflect.TypeOf(v)).(*string)
	*r = v
	return
}
