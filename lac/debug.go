/*
 * Linear Allocator
 *
 * Improve the memory allocation and garbage collection performance.
 *
 * Copyright (C) 2020-2023 crazybie@github.com.
 * https://github.com/crazybie/linear_ac
 */

package lac

import (
	"fmt"
	"math"
	"reflect"
	"sync"
	"unsafe"
)

// Objects in sync.Pool will be recycled on demand by the system (usually after two GC).
// we can put chunks here to make pointers live longer,
// useful to diagnosis use-after-free bugs.
var diagnosisChunkPool = sync.Pool{}

func EnableDebugMode(v bool) {
	debugMode = v
	acPool.Debug = v
	chunkPool.Debug = v

	// reload cfg
	acPool.MaxNew = MaxNewLacInDebug
	acPool.Max = MaxLac
	chunkPool.Max = MaxChunks
}

// DebugCheck check if all items from pool are all returned to pool.
// useful for leak-checking.
func DebugCheck() {
	if debugMode {
		acPool.DebugCheck()

		// chunks will be put to a syncPool instead of chunkPool for debugging purpose.
		// chunkPool.DebugCheck()
	}
}

// CheckExternalPointers is useful for if you want to check external pointers but don't want to invalidate pointers.
// e.g. using lac as memory allocator for config data globally.
func (ac *Allocator) CheckExternalPointers() {
	ac.debugCheck(false)
}

func (ac *Allocator) debugScan(obj any) {
	ac.dbgScanObjsLock.Lock()
	ac.dbgScanObjs = append(ac.dbgScanObjs, obj)
	ac.dbgScanObjsLock.Unlock()
}

// Use 1 instead of nil or MaxUint64 to
// 1. make non-nil check pass.
// 2. generate a recoverable panic.
const nonNilPanickyAddr = uintptr(1)

type PointerType int32

const (
	PointerTypeInvalid PointerType = iota
	PointerTypeLacInternal
	PointerTypeExternal
	PointerTypeExternalMarked
)

func (ac *Allocator) checkPointerType(addr uintptr) PointerType {

	if addr == 0 || addr == nonNilPanickyAddr {
		return PointerTypeLacInternal
	}

	if addr == uintptr(unsafe.Pointer(ac)) {
		return PointerTypeLacInternal
	}

	for _, h := range ac.chunks {
		if addr >= uintptr(h.Data) && addr < uintptr(h.Data)+uintptr(h.Cap) {
			return PointerTypeLacInternal
		}
	}

	for _, c := range ac.externalPtr {
		if uintptr(c) == addr {
			return PointerTypeExternalMarked
		}
	}
	return PointerTypeExternal
}

type CheckCtx struct {
	checked            map[interface{}]struct{}
	unsupportedTypes   map[string]struct{}
	invalidatePointers bool
}

// NOTE: all memories must be referenced by structs.
func (ac *Allocator) debugCheck(invalidatePointers bool) {
	ctx := &CheckCtx{
		checked:            map[interface{}]struct{}{},
		unsupportedTypes:   map[string]struct{}{},
		invalidatePointers: invalidatePointers,
	}

	// reverse order to bypass obfuscated pointers
	for i := len(ac.dbgScanObjs) - 1; i >= 0; i-- {
		ptr := ac.dbgScanObjs[i]
		if _, ok := ctx.checked[ptr]; ok {
			continue
		}
		if err := ac.checkRecursively(reflect.ValueOf(ptr), ctx); err != nil {
			dumpUnsupportedTypes(ctx)
			panic(err)
		}
	}
}

func (ac *Allocator) checkRecursively(val reflect.Value, ctx *CheckCtx) error {
	if val.Kind() == reflect.Ptr {
		if val.Pointer() != nonNilPanickyAddr && !val.IsNil() {
			pt := ac.checkPointerType(val.Pointer())
			if pt == PointerTypeExternal {
				return fmt.Errorf("unexpected external pointer: %+v", val)
			}

			tp := val.Elem().Type()
			if tp == reflect.TypeOf(ac).Elem() {
				// stop scanning Allocator fields.
				return nil
			}

			if pt == PointerTypeLacInternal && tp.Kind() == reflect.Struct {
				if err := ac.checkRecursively(val.Elem(), ctx); err != nil {
					return err
				}
				ctx.checked[interfaceOfUnexported(val)] = struct{}{}
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
			case reflect.Bool,
				reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
				reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
				reflect.Float32, reflect.Float64:
				// no need to check.

			case reflect.Ptr:
				if err := ac.checkRecursively(f, ctx); err != nil {
					return fmt.Errorf("%v: %v", fieldName(i), err)
				}
				if ctx.invalidatePointers {
					*(*uintptr)(unsafe.Pointer(f.UnsafeAddr())) = nonNilPanickyAddr
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
					pt := ac.checkPointerType(uintptr(h.Data))
					if !found && pt == PointerTypeExternal {
						return fmt.Errorf("%s: unexpected external slice: %s", fieldName(i), f.String())
					}
					if pt == PointerTypeLacInternal {
						for j := 0; j < f.Len(); j++ {
							if err := ac.checkRecursively(f.Index(j), ctx); err != nil {
								return fmt.Errorf("%v: %v", fieldName(i), err)
							}
						}
					}
				}
				if ctx.invalidatePointers {
					h.Data = nil
					h.Len = math.MaxInt32
					h.Cap = math.MaxInt32
				}

			case reflect.Array:
				for j := 0; j < f.Len(); j++ {
					if err := ac.checkRecursively(f.Index(j), ctx); err != nil {
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
					if err := ac.checkRecursively(iter.Value(), ctx); err != nil {
						return fmt.Errorf("%v: %v", fieldName(i), err)
					}
				}

			case reflect.String:
				h := (*stringHeader)(unsafe.Pointer(f.UnsafeAddr()))
				if f.Len() > 0 && h.Data != nil {
					found := false
					for _, i := range ac.externalString {
						if i == h.Data {
							found = true
							break
						}
					}
					pt := ac.checkPointerType(uintptr(h.Data))
					if !found && pt == PointerTypeExternal {
						return fmt.Errorf("%s: unexpected external string: %s", fieldName(i), f.String())
					}
				}
				if ctx.invalidatePointers {
					h.Data = nil
					h.Len = math.MaxInt32
				}

			case reflect.Func:
				p := f.UnsafePointer()
				if p != nil {
					found := false
					for _, i := range ac.externalPtr {
						if i == p {
							found = true
							break
						}
					}
					if !found {
						return fmt.Errorf("%s: unexpected external func: %s", fieldName(i), f.String())
					}
				}

			default:
				msg := fmt.Sprintf("WARNING: pointer checking: unsupported type: %v, %v\n", fieldName(i), f.String())
				ctx.unsupportedTypes[msg] = struct{}{}
			}
		}
	}
	return nil
}

var unsupportedTypes = struct {
	sync.Mutex
	m map[string]struct{}
}{m: map[string]struct{}{}}

func dumpUnsupportedTypes(ctx *CheckCtx) {
	unsupportedTypes.Lock()
	for k := range ctx.unsupportedTypes {
		if _, ok := unsupportedTypes.m[k]; !ok {
			unsupportedTypes.m[k] = struct{}{}
			fmt.Printf(k)
		}
	}
	unsupportedTypes.Unlock()
}
