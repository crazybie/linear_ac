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
	"strings"
	"sync"
	"unsafe"
)

// Objects in sync.Pool will be recycled on demand by the system (usually after two GC).
// we can put chunks here to make pointers live longer,
// useful to diagnosis use-after-free bugs.
var diagnosisChunkPool = sync.Pool{}

func (p *AllocatorPool) EnableDebugMode(v bool) {
	p.debugMode = v
	p.Pool.Debug = v
	p.chunkPool.Debug = v

	// reload cfg
	p.MaxNew = MaxNewLacInDebug
	p.Cap = p.MaxLac
	p.chunkPool.Cap = p.chunkPool.MaxChunks
}

func (p *AllocatorPool) DumpStats(reset bool) string {
	chunksUsed := p.Stats.ChunksUsed.Load()
	allocBytes := p.Stats.AllocBytes.Load()
	utilization := float32(allocBytes) / float32(int64(p.chunkPool.ChunkSize)*chunksUsed) * 100

	s := fmt.Sprintf(`
[stats]name:%s,
[alloc]st:%v,mt:%v,bytes:%v,utilization:%.2f,
[chunks]total_new:%v,used:%v,miss:%v,pool:%v,
[lac]total_new:%v,pool:%v`,
		p.Name,
		p.Stats.SingleThreadAlloc.Load(), p.Stats.MultiThreadAlloc.Load(), allocBytes, utilization,
		p.chunkPool.Stats.TotalCreatedChunks.Load(), chunksUsed, p.Stats.ChunksMiss.Load(), len(p.chunkPool.pool),
		p.Stats.TotalCreatedAc.Load(), len(p.pool),
	)
	s = strings.ReplaceAll(s, "\n", "")

	if reset {
		p.Stats.SingleThreadAlloc.Store(0)
		p.Stats.MultiThreadAlloc.Store(0)
		p.Stats.AllocBytes.Store(0)
		p.Stats.ChunksUsed.Store(0)
		p.Stats.ChunksMiss.Store(0)
	}

	return s
}

// DebugCheck check if all items from pool are all returned to pool.
// useful for leak-checking.
func (p *AllocatorPool) DebugCheck() {
	if p.debugMode {
		p.Pool.DebugCheck()

		// chunks will be put to a syncPool instead of chunkPool for debugging purpose.
		// chunkPool.DebugCheck()

		fmt.Printf("Lac: debug check done.\n")
	}
}

// CheckExternalPointers is useful for if you want to check external pointers but don't want to invalidate pointers.
// e.g. using lac as memory allocator for config data globally.
func (ac *Allocator) CheckExternalPointers() {
	ac.debugCheck(false)
}

func (ac *Allocator) debugScan(obj any) {
	ac.dbgScanObjs.Put(obj)
}

// Use 1 instead of nil or MaxUint64 to
// 1. make non-nil check pass to allow the dereference of pointer.
// 2. generate a recoverable panic.
const nonNilPanickyAddr = uintptr(1)

type pointerType int32

const (
	pointerTypeInvalid pointerType = iota
	pointerTypeLacInternal
	pointerTypeExternal
	pointerTypeExternalMarked
)

func (ac *Allocator) checkPointerType(addr uintptr) pointerType {

	if addr == 0 || addr == nonNilPanickyAddr {
		return pointerTypeLacInternal
	}

	if addr == uintptr(unsafe.Pointer(ac)) {
		return pointerTypeLacInternal
	}

	for _, h := range ac.chunks {
		if addr >= uintptr(h.Data) && addr < uintptr(h.Data)+uintptr(h.Cap) {
			return pointerTypeLacInternal
		}
	}

	for _, c := range ac.externalPtr.slice {
		if uintptr(c) == addr {
			return pointerTypeExternalMarked
		}
	}
	return pointerTypeExternal
}

type checkCtx struct {
	checked            map[interface{}]struct{}
	unsupportedTypes   map[string]struct{}
	invalidatePointers bool
}

// NOTE: all memories must be referenced by structs.
func (ac *Allocator) debugCheck(invalidatePointers bool) {
	ctx := &checkCtx{
		checked:            map[interface{}]struct{}{},
		unsupportedTypes:   map[string]struct{}{},
		invalidatePointers: invalidatePointers,
	}

	// reverse order to bypass obfuscated pointers
	for i := len(ac.dbgScanObjs.slice) - 1; i >= 0; i-- {
		ptr := ac.dbgScanObjs.slice[i]
		if _, ok := ctx.checked[ptr]; ok {
			continue
		}
		if err := ac.checkRecursively(reflect.ValueOf(ptr), ctx); err != nil {
			dumpUnsupportedTypes(ctx)
			panic(err)
		}
	}
}

func (ac *Allocator) checkRecursively(val reflect.Value, ctx *checkCtx) error {
	if val.Kind() == reflect.Ptr {
		if val.Pointer() != nonNilPanickyAddr && !val.IsNil() {
			pt := ac.checkPointerType(val.Pointer())
			if pt == pointerTypeExternal {
				return fmt.Errorf("unexpected external pointer: %+v", val)
			}

			tp := val.Elem().Type()
			if tp == reflect.TypeOf(ac).Elem() {
				// stop scanning Allocator fields.
				return nil
			}

			if pt == pointerTypeLacInternal && tp.Kind() == reflect.Struct {
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
					for _, s := range ac.externalSlice.slice {
						if s == h.Data {
							found = true
							break
						}
					}
					pt := ac.checkPointerType(uintptr(h.Data))
					if !found && pt == pointerTypeExternal {
						return fmt.Errorf("%s: unexpected external slice: %s", fieldName(i), f.String())
					}
					if pt == pointerTypeLacInternal {
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
				for _, i := range ac.externalMap.slice {
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
					for _, s := range ac.externalString.slice {
						if s == h.Data {
							found = true
							break
						}
					}
					pt := ac.checkPointerType(uintptr(h.Data))
					if !found && pt == pointerTypeExternal {
						return fmt.Errorf("%s: unexpected external string: %s", fieldName(i), f.String())
					}
				}
				if ctx.invalidatePointers {
					h.Data = nil
					h.Len = math.MaxInt32
				}

			case reflect.Func:
				p := interfaceOfUnexported(f)
				if data(p) != nil {
					found := false
					for _, i2 := range ac.externalFunc.slice {
						if interfaceEqual(i2, p) {
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

func dumpUnsupportedTypes(ctx *checkCtx) {
	unsupportedTypes.Lock()
	for k := range ctx.unsupportedTypes {
		if _, ok := unsupportedTypes.m[k]; !ok {
			unsupportedTypes.m[k] = struct{}{}
			fmt.Printf(k)
		}
	}
	unsupportedTypes.Unlock()
}
