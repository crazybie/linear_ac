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
	disabled        bool
	chunks          []*sliceHeader
	curChunk        unsafe.Pointer //*sliceHeader
	refCnt          int32
	TotalAllocBytes int32

	// NOTE:
	// To keep these externals alive, slices must be alloc from raw allocator to make them
	// available to the GC. never alloc them from Lac itself.
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
		chunks:   make([]*sliceHeader, 0, 4),
	}
	return ac
}

// alloc use lock-free algorithm to avoid locking.
func (ac *Allocator) alloc(need int, zero bool) unsafe.Pointer {
	needAligned := (need + PtrSize + 1) & ^(PtrSize - 1)
	var header, new_ *sliceHeader
	var len_, cap_ int64

	// fast version for single-threaded case.
	if atomic.LoadInt32(&ac.refCnt) == 1 {
		for {
			if ac.curChunk != nil {
				header = (*sliceHeader)(ac.curChunk)
				len_ = header.Len
				cap_ = header.Cap
			}
			if len_+int64(needAligned) > cap_ {
				if needAligned > ChunkSize {
					t := make(chunk, 0, need)
					new_ = (*sliceHeader)(unsafe.Pointer(&t))
				} else {
					new_ = chunkPool.Get()
				}
				ac.curChunk = unsafe.Pointer(new_)
				ac.chunks = append(ac.chunks, new_)
			} else {
				header.Len += int64(needAligned)
				ptr := unsafe.Add(header.Data, len_)
				if zero {
					memclrNoHeapPointers(ptr, uintptr(needAligned))
				}
				ac.TotalAllocBytes += int32(needAligned)
				return ptr
			}
		}
	}

	// lock-free version for multi-threaded case.
	for {
		cur := atomic.LoadPointer(&ac.curChunk)
		if cur != nil {
			header = (*sliceHeader)(cur)
			len_ = atomic.LoadInt64(&header.Len)
			cap_ = header.Cap
		}

		if len_+int64(needAligned) > cap_ {
			if needAligned > ChunkSize {
				t := make(chunk, 0, need)
				new_ = (*sliceHeader)(unsafe.Pointer(&t))
			} else {
				new_ = chunkPool.Get()
			}
			if atomic.CompareAndSwapPointer(&ac.curChunk, cur, unsafe.Pointer(new_)) {
				ac.chunks = append(ac.chunks, new_)
			} else if new_.Cap == int64(ChunkSize) {
				chunkPool.Put(new_)
			}
		} else {
			if atomic.CompareAndSwapInt64(&header.Len, len_, len_+int64(needAligned)) {
				ptr := unsafe.Add(header.Data, len_)
				if zero {
					memclrNoHeapPointers(ptr, uintptr(needAligned))
				}
				atomic.AddInt32(&ac.TotalAllocBytes, int32(needAligned))
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
	ac.chunks = resetSlice(ac.chunks)
	ac.curChunk = nil

	// clear externals
	ac.externalPtr = nil
	ac.externalSlice = nil
	ac.externalMap = nil
	ac.externalString = nil

	ac.disabled = DisableLac
	atomic.StoreInt32(&ac.refCnt, 1)
	ac.TotalAllocBytes = 0
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
		defer ac.externalPtrLock.Unlock()
		ac.externalPtr = append(ac.externalPtr, d)
	case reflect.Slice:
		ac.externalSliceLock.Lock()
		defer ac.externalSliceLock.Unlock()
		ac.externalSlice = append(ac.externalSlice, (*sliceHeader)(d).Data)
	case reflect.String:
		ac.externalStringLock.Lock()
		defer ac.externalStringLock.Unlock()
		ac.externalString = append(ac.externalString, (*stringHeader)(d).Data)
	case reflect.Map:
		ac.externalMapLock.Lock()
		defer ac.externalMapLock.Unlock()
		ac.externalMap = append(ac.externalMap, d)
	case reflect.Func:
		ac.externalPtrLock.Lock()
		defer ac.externalPtrLock.Unlock()
		ac.externalPtr = append(ac.externalPtr, reflect.ValueOf(ptr).UnsafePointer())
	}
}
