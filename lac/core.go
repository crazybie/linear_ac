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
	"sync/atomic"
	"unsafe"
)

// Chunk Pool

type chunk []byte

type ChunkPool struct {
	Pool[*sliceHeader]

	ChunkSize int
	MaxChunks int

	Stats struct {
		TotalCreatedChunks atomic.Int64
	}
}

func NewChunkPool(name string, chunkSz, defaultChunks, maxChunks int) *ChunkPool {
	r := &ChunkPool{
		Pool: Pool[*sliceHeader]{
			Name:  fmt.Sprintf("LacChunkPool(%s)", name),
			Equal: func(a, b *sliceHeader) bool { return a == b },
		},
		ChunkSize: chunkSz,
		MaxChunks: maxChunks,
	}

	r.New = func() *sliceHeader {
		c := make(chunk, 0, chunkSz)
		r.Stats.TotalCreatedChunks.Add(1)
		return (*sliceHeader)(unsafe.Pointer(&c))
	}
	r.Pool.Cap = r.MaxChunks
	r.Reserve(defaultChunks)

	return r
}

// Allocator Pool

type AllocatorPool struct {
	Pool[*Allocator]
	MaxLac    int
	chunkPool *ChunkPool
	Name      string
	debugMode bool

	Stats struct {
		TotalCreatedAc    atomic.Int64
		SingleThreadAlloc atomic.Int64
		MultiThreadAlloc  atomic.Int64
		TotalAllocBytes   atomic.Int64
		ChunksUsed        atomic.Int64
		ChunksMiss        atomic.Int64
	}
}

func NewAllocatorPool(name string, poolCap, chunkSz, defaultChunks, maxChunks int) *AllocatorPool {
	chunkPool := NewChunkPool(name, chunkSz, defaultChunks, maxChunks)

	r := &AllocatorPool{
		Name:      name,
		chunkPool: chunkPool,
		Pool: Pool[*Allocator]{
			Name:   fmt.Sprintf("LacPool(%s)", name),
			Cap:    poolCap,
			Equal:  func(a, b *Allocator) bool { return a == b },
			MaxNew: MaxNewLacInDebug,
		},
	}
	r.Pool.New = func() *Allocator { return newLac(r) }

	return r
}

// Allocator

type Allocator struct {
	disabled        bool
	chunks          []*sliceHeader
	chunksLock      SpinLock
	curChunk        unsafe.Pointer //*sliceHeader
	refCnt          int32
	TotalAllocBytes int32
	acPool          *AllocatorPool

	// NOTE:
	// To keep these externals alive, slices must be alloc from raw allocator to make them
	// available to the GC. never alloc them from Lac itself.
	externalPtr    WeakUniqQueue[unsafe.Pointer]
	externalSlice  WeakUniqQueue[unsafe.Pointer]
	externalString WeakUniqQueue[unsafe.Pointer]
	externalMap    WeakUniqQueue[any]
	externalFunc   WeakUniqQueue[any]

	dbgScanObjs WeakUniqQueue[any]
}

func newLac(acPool *AllocatorPool) *Allocator {
	ac := &Allocator{
		disabled: DisableAllLac,
		refCnt:   1,
		chunks:   make([]*sliceHeader, 0, 4),
		acPool:   acPool,

		externalPtr:    NewWeakUniqQueue(32, unsafePtrEq),
		externalSlice:  NewWeakUniqQueue(32, unsafePtrEq),
		externalString: NewWeakUniqQueue(32, unsafePtrEq),
		externalMap:    NewWeakUniqQueue(32, anyEq),
		externalFunc:   NewWeakUniqQueue(32, interfaceEqual),

		dbgScanObjs: NewWeakUniqQueue(math.MaxInt, anyEq),
	}

	acPool.Stats.TotalCreatedAc.Add(1)
	return ac
}

// alloc auto select single-thread or multi-thread algo.
// multi-thread version uses lock-free algorithm to reduce locking.
func (ac *Allocator) alloc(need int, zero bool) unsafe.Pointer {
	needAligned := (need + PtrSize + 1) & ^(PtrSize - 1)
	chunkPool := ac.acPool.chunkPool
	var header, new_ *sliceHeader
	var len_, cap_ int64

	// single-threaded path
	if atomic.LoadInt32(&ac.refCnt) == 1 {
		ac.acPool.Stats.SingleThreadAlloc.Add(1)

		for {
			if ac.curChunk != nil {
				header = (*sliceHeader)(ac.curChunk)
				len_ = header.Len
				cap_ = header.Cap
			}
			if len_+int64(needAligned) > cap_ {
				if needAligned > chunkPool.ChunkSize {
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
				ac.acPool.Stats.TotalAllocBytes.Add(int64(needAligned))
				return ptr
			}
		}
	}

	// multi-threaded path
	ac.acPool.Stats.MultiThreadAlloc.Add(1)
	for {
		cur := atomic.LoadPointer(&ac.curChunk)
		if cur != nil {
			header = (*sliceHeader)(cur)
			len_ = atomic.LoadInt64(&header.Len)
			cap_ = header.Cap
		}

		if len_+int64(needAligned) > cap_ {
			if needAligned > chunkPool.ChunkSize {
				t := make(chunk, 0, need)
				new_ = (*sliceHeader)(unsafe.Pointer(&t))
			} else {
				new_ = chunkPool.Get()
			}
			if atomic.CompareAndSwapPointer(&ac.curChunk, cur, unsafe.Pointer(new_)) {
				ac.chunksLock.Lock()
				ac.chunks = append(ac.chunks, new_)
				ac.chunksLock.Unlock()
			} else if new_.Cap == int64(chunkPool.ChunkSize) {
				chunkPool.Put(new_)
			}
		} else {
			if atomic.CompareAndSwapInt64(&header.Len, len_, len_+int64(needAligned)) {
				ptr := unsafe.Add(header.Data, len_)
				if zero {
					memclrNoHeapPointers(ptr, uintptr(needAligned))
				}
				atomic.AddInt32(&ac.TotalAllocBytes, int32(needAligned))
				ac.acPool.Stats.TotalAllocBytes.Add(int64(needAligned))
				return ptr
			}
		}
	}
}

func (ac *Allocator) reset() {
	if ac.disabled {
		return
	}

	if ac.acPool.debugMode {
		ac.debugCheck(true)
		ac.dbgScanObjs.Clear()
	}

	stats := &ac.acPool.Stats

	for _, ck := range ac.chunks {
		ck.Len = 0

		// only reuse the normal chunks,
		// otherwise we may have too many large chunks wasted.
		if ck.Cap == int64(ac.acPool.chunkPool.ChunkSize) {
			stats.ChunksUsed.Add(1)

			if ac.acPool.debugMode {
				diagnosisChunkPool.Put(ck)
			} else {
				ac.acPool.chunkPool.Put(ck)
			}
		} else {
			if ac.acPool.debugMode {
				diagnosisChunkPool.Put(ck)
			} else {
				// recycle by GC.
			}
			stats.ChunksMiss.Add(1)
		}
	}

	// clear all ref
	ac.chunks = resetSlice(ac.chunks)
	ac.curChunk = nil

	// clear externals
	ac.externalPtr.Clear()
	ac.externalSlice.Clear()
	ac.externalMap.Clear()
	ac.externalString.Clear()
	ac.externalFunc.Clear()

	ac.disabled = DisableAllLac
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

	k := reflect.TypeOf(ptr).Kind()
	switch k {
	case reflect.Ptr:
		ac.externalPtr.Put(d)
	case reflect.Slice:
		ac.externalSlice.Put((*sliceHeader)(d).Data)
	case reflect.String:
		ac.externalString.Put((*stringHeader)(d).Data)
	case reflect.Map:
		ac.externalMap.Put(d)
	case reflect.Func:
		ac.externalFunc.Put(ptr)
	default:
		panic(fmt.Errorf("unsupported type: %v", k))
	}
}
