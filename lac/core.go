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
	Stats     struct {
		TotalCreated atomic.Int64
	}
}

func newChunkPool(name string, logger Logger, chunkSz, defaultChunks, chunksCap int) *ChunkPool {
	r := &ChunkPool{
		Pool: Pool[*sliceHeader]{
			Logger: logger,
			Name:   fmt.Sprintf("LacChunkPool(%s)", name),
			Equal:  eq[*sliceHeader],
			Cap:    chunksCap,
		},
		ChunkSize: chunkSz,
	}

	r.New = func() *sliceHeader {
		c := make(chunk, 0, chunkSz)
		r.Stats.TotalCreated.Add(1)
		return (*sliceHeader)(unsafe.Pointer(&c))
	}

	r.Reserve(defaultChunks)

	return r
}

// Allocator Pool

type AllocatorPool struct {
	Logger
	Pool[*Allocator]

	debugMode bool
	MaxLac    int
	chunkPool *ChunkPool
	Name      string

	Stats struct {
		TotalCreatedAc    atomic.Int64
		SingleThreadAlloc atomic.Int64
		MultiThreadAlloc  atomic.Int64
		AllocBytes        atomic.Int64
		ChunksUsed        atomic.Int64
		ChunksMiss        atomic.Int64
	}
}

func NewAllocatorPool(name string, logger Logger, lacCap, chunkSz, defaultChunks, chunksCap int) *AllocatorPool {
	chunkPool := newChunkPool(name, logger, chunkSz, defaultChunks, chunksCap)

	r := &AllocatorPool{
		Name:      name,
		Logger:    logger,
		chunkPool: chunkPool,
		Pool: Pool[*Allocator]{
			Name:   fmt.Sprintf("LacPool(%s)", name),
			Cap:    lacCap,
			Equal:  eq[*Allocator],
			MaxNew: lacCap * 100,
		},
	}
	r.Pool.New = func() *Allocator { return newLac(r) }

	return r
}

// Allocator

type Allocator struct {
	disabled   bool
	valid      bool
	refCnt     atomic.Int32
	chunks     []*sliceHeader
	chunksLock spinLock
	curChunk   unsafe.Pointer //*sliceHeader
	acPool     *AllocatorPool

	// NOTE:
	// To keep these externals alive, slices must be alloc from raw allocator to make them
	// available to the GC. never alloc them from Lac itself.
	externalPtr    weakUniqQueue[unsafe.Pointer]
	externalSlice  weakUniqQueue[unsafe.Pointer]
	externalString weakUniqQueue[unsafe.Pointer]
	externalMap    weakUniqQueue[any]
	externalFunc   weakUniqQueue[any]

	dbgScanObjs weakUniqQueue[any]
}

func newLac(acPool *AllocatorPool) *Allocator {
	ac := &Allocator{
		disabled: DisableAllLac,
		chunks:   make([]*sliceHeader, 0, 4),
		acPool:   acPool,

		externalPtr:    newWeakUniqQueue(32, eq[unsafe.Pointer]),
		externalSlice:  newWeakUniqQueue(32, eq[unsafe.Pointer]),
		externalString: newWeakUniqQueue(32, eq[unsafe.Pointer]),
		externalMap:    newWeakUniqQueue(32, anyEq),
		externalFunc:   newWeakUniqQueue(32, interfaceEqual),

		dbgScanObjs: newWeakUniqQueue(math.MaxInt, anyEq),
	}

	ac.refCnt.Store(1)

	acPool.Stats.TotalCreatedAc.Add(1)
	return ac
}

// alloc auto select single-thread or multi-thread algo.
// multi-thread version uses lock-free algorithm to reduce locking.
func (ac *Allocator) alloc(need int, zero bool) unsafe.Pointer {
	needAligned := (need + ptrSize + 1) & ^(ptrSize - 1)
	chunkPool := ac.acPool.chunkPool
	stats := &ac.acPool.Stats
	var header, new_ *sliceHeader
	var len_, cap_ int64

	ac.checkValidity()

	// single-threaded path
	if ac.refCnt.Load() == 1 {
		stats.SingleThreadAlloc.Add(1)

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
				stats.AllocBytes.Add(int64(needAligned))
				return ptr
			}
		}
	}

	// multi-threaded path
	stats.MultiThreadAlloc.Add(1)
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
				stats.AllocBytes.Add(int64(needAligned))
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
	ac.valid = false
	ac.refCnt.Store(1)
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
