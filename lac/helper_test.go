/*
 * Linear Allocator
 *
 * Improve the memory allocation and garbage collection performance.
 *
 * Copyright (C) 2020-2022 crazybie@github.com.
 * https://github.com/crazybie/linear_ac
 */

package lac

import (
	"runtime"
	"testing"
	"unsafe"
)

func TestAllocator_AttachExternalNoAlloc(t *testing.T) {
	ac := Get()
	ac.externalPtr = make([]unsafe.Pointer, 0, 4)
	defer ac.Release()

	s := new(int)
	noMalloc(func() {
		AttachExternal(ac, s)
	})
}

func TestAllocator_AttachExternalIface(t *testing.T) {
	ac := Get()
	ac.externalPtr = make([]unsafe.Pointer, 0, 4)
	defer ac.Release()

	i := new(int)
	noMalloc(func() {
		var v interface{} = i
		AttachExternal(ac, v)
	})
}

func TestLinearAllocator_NewExternalPtr(b *testing.T) {
	ac := Get()
	defer ac.Release()

	type D struct {
		d [10]*int
	}
	d := New[D](ac)
	for i := 0; i < len(d.d); i++ {
		d.d[i] = AttachExternal(ac, new(int))
		//d.d[i] = new(int)
		*d.d[i] = i
		runtime.GC()
	}

	for i := 0; i < len(d.d); i++ {
		if *d.d[i] != i {
			b.Errorf("should not be gced.")
		}
	}
}

func Test_GenericAppend(t *testing.T) {
	ac := Get()
	defer ac.Release()

	m := map[int][]int{}

	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			// has 1 malloc after Append returns.
			m[i] = Append(ac, m[i], j)
		}
	}

	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			if m[i][j] != j {
				t.Fail()
			}
		}
	}
}

func Test_AppendNoMalloc(t *testing.T) {
	chunkPool.reserve(1)
	ac := Get()
	defer ac.Release()

	m := map[int][]int{}
	// init map buckets
	for i := 0; i < 10; i++ {
		m[i] = nil
	}

	noMalloc(func() {
		for i := 0; i < 10; i++ {
			for j := 0; j < 10; j++ {
				s := m[i]
				ac.SliceAppend(&s, j)
				m[i] = s
			}
		}
	})
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			if m[i][j] != j {
				t.Fail()
			}
		}
	}
}
