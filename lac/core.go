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
	"reflect"
	"sync/atomic"
	"unsafe"
)

// Chunk

type chunk []byte

var chunkPool = Pool[*sliceHeader]{
	New: func() *sliceHeader {
		c := make(chunk, 0, ChunkSize)
		return (*sliceHeader)(unsafe.Pointer(&c))
	},
	Max:   MaxChunks,
	Equal: func(a, b *sliceHeader) bool { return unsafe.Pointer(a) == unsafe.Pointer(b) },
}

// Allocator

type Allocator struct {
	disabled   bool
	chunks     []*sliceHeader
	chunksLock SpinLock
	curChunk   unsafe.Pointer //*sliceHeader
	refCnt     int32

	externalPtr        []unsafe.Pointer
	externalPtrLock    SpinLock
	externalSlice      []unsafe.Pointer
	externalSliceLock  SpinLock
	externalString     []unsafe.Pointer
	externalStringLock SpinLock
	externalMap        []interface{}
	externalMapLock    SpinLock
	dbgScanObjs        []interface{}
	dbgScanObjsLock    SpinLock
}

func newLac() *Allocator {
	ac := &Allocator{
		disabled: DisableLac,
		refCnt:   1,
	}
	return ac
}

// alloc use lock-free algorithm to avoid locking.
func (ac *Allocator) alloc(need int, zero bool) unsafe.Pointer {
	needAligned := (need + PtrSize + 1) & ^(PtrSize - 1)
	var header *sliceHeader
	var len_, cap_ int64

	for {
		cur := atomic.LoadPointer(&ac.curChunk)
		if cur != nil {
			header = (*sliceHeader)(cur)
			len_ = atomic.LoadInt64(&header.Len)
			cap_ = atomic.LoadInt64(&header.Cap)
		}

		if len_+int64(needAligned) > cap_ {
			var new_ *sliceHeader
			fromPool := true
			if needAligned > ChunkSize {
				// this heap object may be wasted due to the cas failure.
				// this is where wait-free algo is better.
				t := make(chunk, 0, need)
				new_ = (*sliceHeader)(unsafe.Pointer(&t))
				fromPool = false
			} else {
				new_ = chunkPool.Get()
			}
			if atomic.CompareAndSwapPointer(&ac.curChunk, cur, unsafe.Pointer(new_)) {
				ac.chunksLock.Lock()
				ac.chunks = append(ac.chunks, new_)
				ac.chunksLock.Unlock()
			} else {
				if fromPool {
					chunkPool.Put(new_)
				}
			}
		} else {
			if atomic.CompareAndSwapInt64(&header.Len, len_, len_+int64(needAligned)) {
				ptr := unsafe.Add(atomic.LoadPointer(&header.Data), len_)
				if zero {
					memclrNoHeapPointers(ptr, uintptr(needAligned))
				}
				return ptr
			}
		}
	}
}

func (ac *Allocator) reset() {
	if ac.disabled {
		return
	}

	if debugMode {
		ac.debugCheck(true)
		ac.dbgScanObjs = ac.dbgScanObjs[:0]
	}

	for _, ck := range ac.chunks {
		ck.Len = 0
		if debugMode {
			diagnosisChunkPool.Put(ck)
		} else {
			// only reuse the normal chunks,
			// otherwise we may have too many large chunks wasted.
			if ck.Cap == int64(ChunkSize) {
				chunkPool.Put(ck)
			}
		}
	}

	// clear all ref
	ac.chunks = ac.chunks[:cap(ac.chunks)]
	for i := 0; i < cap(ac.chunks); i++ {
		ac.chunks[i] = nil
	}
	ac.chunks = ac.chunks[:0]
	ac.curChunk = nil

	// clear externals
	ac.externalPtr = nil
	ac.externalSlice = nil
	ac.externalMap = nil

	ac.disabled = DisableLac
	atomic.StoreInt32(&ac.refCnt, 1)
}

func (ac *Allocator) typedAlloc(ptrTp reflect.Type, sz uintptr, zero bool) (ret interface{}) {
	if sz == 0 {
		sz = ptrTp.Elem().Size()
	}
	ptr := ac.alloc(int(sz), zero)
	*(*emptyInterface)(unsafe.Pointer(&ret)) = emptyInterface{data(ptrTp), ptr}

	if debugMode {
		if ptrTp.Elem().Kind() == reflect.Struct {
			ac.dbgScanObjsLock.Lock()
			ac.dbgScanObjs = append(ac.dbgScanObjs, ret)
			ac.dbgScanObjsLock.Unlock()
		}
	}

	return
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
		ac.externalPtrLock.Lock()
		ac.externalPtr = append(ac.externalPtr, d)
		ac.externalPtrLock.Unlock()
	case reflect.Slice:
		ac.externalSliceLock.Lock()
		ac.externalSlice = append(ac.externalSlice, (*sliceHeader)(d).Data)
		ac.externalSliceLock.Unlock()
	case reflect.String:
		ac.externalStringLock.Lock()
		ac.externalString = append(ac.externalString, (*stringHeader)(d).Data)
		ac.externalStringLock.Unlock()
	case reflect.Map:
		ac.externalMapLock.Lock()
		ac.externalMap = append(ac.externalMap, d)
		ac.externalMapLock.Unlock()
	case reflect.Func:
		ac.externalPtrLock.Lock()
		ac.externalPtr = append(ac.externalPtr, reflect.ValueOf(ptr).UnsafePointer())
		ac.externalPtrLock.Unlock()
	}
}
