/*
 * Linear Allocator
 *
 * Improve the memory allocation and garbage collection performance.
 *
 * Copyright (C) 2020-2021 crazybie@github.com.
 * https://github.com/crazybie/linear_ac
 */

package lac

import (
	"fmt"
	"math"
	"reflect"
	"unsafe"
)

// Use 1 instead of nil or MaxUint64 to
// 1. make non-nil check pass.
// 2. generate a recoverable panic.
const nonNilPanicableAddr = uintptr(1)

func (ac *Allocator) internalPointer(addr uintptr) bool {

	if addr == 0 || addr == nonNilPanicableAddr {
		return true
	}

	if addr == uintptr(unsafe.Pointer(ac)) {
		return true
	}

	for _, c := range ac.chunks {
		h := (*sliceHeader)(unsafe.Pointer(c))
		if addr >= uintptr(h.Data) && addr < uintptr(h.Data)+uintptr(h.Cap) {
			return true
		}
	}

	for _, c := range ac.externalPtr {
		if uintptr(c) == addr {
			return true
		}
	}
	return false
}

// NOTE: all memories must be referenced by structs.
func (ac *Allocator) debugCheck(invalidatePointers bool) {
	checked := map[interface{}]struct{}{}
	// reverse order to bypass obfuscated pointers
	for i := len(ac.dbgScanObjs) - 1; i >= 0; i-- {
		ptr := ac.dbgScanObjs[i]
		if _, ok := checked[ptr]; ok {
			continue
		}
		if err := ac.checkRecursively(reflect.ValueOf(ptr), checked, invalidatePointers); err != nil {
			panic(err)
		}
	}
}

// CheckExternalPointers is useful for if you want to check external pointers but don't want to invalidate pointers.
// e.g. using lac as memory allocator for config data globally.
func (ac *Allocator) CheckExternalPointers() {
	ac.debugCheck(false)
}

func (ac *Allocator) checkRecursively(val reflect.Value, checked map[interface{}]struct{}, invalidatePointers bool) error {
	if val.Kind() == reflect.Ptr {
		if val.Pointer() != nonNilPanicableAddr && !val.IsNil() {
			if !ac.internalPointer(val.Pointer()) {
				return fmt.Errorf("unexpected external pointer: %+v", val)
			}

			tp := val.Elem().Type()
			if tp == reflect.TypeOf(ac).Elem() {
				// stop scanning Allocator fields.
				return nil
			}

			if tp.Kind() == reflect.Struct {
				if err := ac.checkRecursively(val.Elem(), checked, invalidatePointers); err != nil {
					return err
				}
				checked[interfaceOfUnexported(val)] = struct{}{}
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
					*(*uintptr)(unsafe.Pointer(f.UnsafeAddr())) = nonNilPanicableAddr
				}

			case reflect.Slice:
				h := (*sliceHeader)(unsafe.Pointer(f.UnsafeAddr()))
				if f.Len() > 0 && h.Data != nil {
					found := false
					for _, i := range ac.externalSlice {
						if i == h.Data {
							found = true
							break
						}
					}
					if !found && !ac.internalPointer((uintptr)(h.Data)) {
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
				found := false
				for _, i := range ac.externalMap {
					if data(i) == m {
						found = true
						break
					}
				}
				if !found {
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
