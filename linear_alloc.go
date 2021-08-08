package auto_pb

import (
	"fmt"
	"reflect"
	"unsafe"
)

type LinearAllocator struct {
	mem []byte
}

func NewLinearAllocator(cap int32) *LinearAllocator {
	return &LinearAllocator{
		mem: make([]byte, 0, cap),
	}
}

func (ac *LinearAllocator) FreeAll() {
	ac.mem = ac.mem[:0]
}

func (ac *LinearAllocator) New(ptrToPtr interface{}) {
	tp := reflect.TypeOf(ptrToPtr).Elem().Elem()
	reflect.ValueOf(ptrToPtr).Elem().Set(reflect.ValueOf(ac.Alloc(tp)))
}

func (ac *LinearAllocator) Alloc(tp reflect.Type) interface{} {
	used, need := len(ac.mem), int(tp.Size())
	if used+need > cap(ac.mem) {
		panic(fmt.Errorf("not enough space"))
	}

	ac.mem = ac.mem[:used+need]
	ptr := unsafe.Pointer(&ac.mem[used])
	for i := 0; i < need; i++ {
		ac.mem[i+used] = 0
	}

	return reflect.NewAt(tp, ptr).Interface()
}

func (ac *LinearAllocator) Bool(v bool) *bool {
	return ac.Alloc(reflect.TypeOf(v)).(*bool)
}

func (ac *LinearAllocator) Int(v int) *int {
	return ac.Alloc(reflect.TypeOf(v)).(*int)
}

func (ac *LinearAllocator) Int32(v int32) *int32 {
	return ac.Alloc(reflect.TypeOf(v)).(*int32)
}

func (ac *LinearAllocator) Int64(v int64) *int64 {
	return ac.Alloc(reflect.TypeOf(v)).(*int64)
}

func (ac *LinearAllocator) Float32(v float32) *float32 {
	return ac.Alloc(reflect.TypeOf(v)).(*float32)
}

func (ac *LinearAllocator) Float64(v float64) *float64 {
	return ac.Alloc(reflect.TypeOf(v)).(*float64)
}

func (ac *LinearAllocator) String(v string) *string {
	return ac.Alloc(reflect.TypeOf(v)).(*string)
}
