/*
 * Linear Allocator
 *
 * Improve the memory allocation and garbage collection performance.
 *
 * Copyright (C) 2020-2022 crazybie@github.com.
 * https://github.com/crazybie/linear_ac
 */

package gls

import (
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"
)

//============================================================================
// GoroutineId
//============================================================================

// https://notes.volution.ro/v1/2019/08/notes/23e3644e/

var goRoutineIdOffset uint64

func goRoutinePtr() unsafe.Pointer

func GoRoutineId() uint64 {
	d := (*[32]uint64)(goRoutinePtr())
	if offset := atomic.LoadUint64(&goRoutineIdOffset); offset != 0 {
		return d[int(offset)]
	}
	id := goRoutineIdSlow()
	var n, offset int
	for idx, v := range d[:] {
		if v == id {
			offset = idx
			n++
			if n >= 2 {
				break
			}
		}
	}
	if n == 1 {
		atomic.StoreUint64(&goRoutineIdOffset, uint64(offset))
	}
	return id
}

func goRoutineIdSlow() uint64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	stk := strings.TrimPrefix(string(buf[:n]), "goroutine ")
	if id, err := strconv.Atoi(strings.Fields(stk)[0]); err != nil {
		panic(err)
	} else {
		return uint64(id)
	}
}

//============================================================================
// GLS: Goroutine Local Storage
//============================================================================

type Gls[T any] struct {
	lk       sync.RWMutex
	m        map[uint64]T
	createFn func() T
}

func NewGls[T any](createFn func() T) *Gls[T] {
	return &Gls[T]{
		m:        map[uint64]T{},
		createFn: createFn,
	}
}

type GetOpt[T any] struct {
	validateFn func(T) bool
	createFn   func() T
}

type GetOptFn[T any] func(opt *GetOpt[T])

func WithValidateFn[T any](f func(T) bool) GetOptFn[T] {
	return func(o *GetOpt[T]) {
		o.validateFn = f
	}
}

func WithCreateFn[T any](f func() T) GetOptFn[T] {
	return func(o *GetOpt[T]) {
		o.createFn = f
	}
}

func (g *Gls[T]) Set(v T) {
	k := GoRoutineId()
	g.lk.Lock()
	defer g.lk.Unlock()
	g.m[k] = v
}

func (g *Gls[T]) Get(opts ...GetOptFn[T]) T {
	k := GoRoutineId()

	optObj := &GetOpt[T]{}
	for _, f := range opts {
		f(optObj)
	}

	g.lk.RLock()
	if v, ok := g.m[k]; ok {
		if optObj.validateFn == nil || optObj.validateFn(v) {
			g.lk.RUnlock()
			return v
		}
	}
	g.lk.RUnlock()

	g.lk.Lock()
	defer g.lk.Unlock()
	// cache breakdown protection
	if v, ok := g.m[k]; ok {
		if optObj.validateFn == nil || optObj.validateFn(v) {
			return v
		}
	}

	var v T
	if optObj.createFn != nil {
		v = optObj.createFn()
	} else if g.createFn != nil {
		v = g.createFn()
	}
	g.m[k] = v
	return v
}
