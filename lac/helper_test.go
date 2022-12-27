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

func TestAllocator_ExternalPtrNoAlloc(t *testing.T) {
	ac := Get()
	defer ac.Release()
	s := new(int)
	ac.externalPtr = make([]unsafe.Pointer, 0, 4)
	NoMalloc(func() {
		AttachExternalPtr(ac, s)
	})
}

func TestAllocator_ExternalSliceNoAlloc(t *testing.T) {
	ac := Get()
	defer ac.Release()
	s := make([]int, 1)
	ac.externalSlice = make([]unsafe.Pointer, 0, 4)
	NoMalloc(func() {
		AttachExternalSlice(ac, s)
	})
}
