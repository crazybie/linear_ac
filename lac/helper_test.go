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
	"testing"
	"unsafe"
)

func TestAllocator_AttachExternalNoAlloc(t *testing.T) {
	ac := Get()
	ac.externalPtr = make([]unsafe.Pointer, 0, 4)
	defer ac.Release()

	s := new(int)
	NoMalloc(func() {
		AttachExternal(ac, s)
	})
}

func TestAllocator_AttachExternalSliceNoAlloc(t *testing.T) {
	ac := Get()
	ac.externalSlice = make([]unsafe.Pointer, 0, 4)
	defer ac.Release()

	s := make([]int, 1)
	NoMalloc(func() {
		AttachExternal(ac, s)
	})
}

func TestAllocator_AttachExternalIface(t *testing.T) {
	ac := Get()
	ac.externalPtr = make([]unsafe.Pointer, 0, 4)
	defer ac.Release()

	i := new(int)
	NoMalloc(func() {
		var v interface{} = i
		AttachExternal(ac, v)
	})
}
