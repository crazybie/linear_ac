/*
 * Linear Allocator
 *
 * Improve the memory allocation and garbage collection performance.
 *
 * Copyright (C) 2020-2023 crazybie@github.com.
 * https://github.com/crazybie/linear_ac
 */

package lac

import "testing"

func Test_PoolDebug(t *testing.T) {
	p := Pool[int]{
		Debug: true,
		New:   func() int { return 0 },
		Equal: func(a, b int) bool { return a == b },
	}
	defer func() {
		if err := recover(); err == nil {
			panic("duplicated item not detected")
		}
	}()
	p.Put(1)
	p.Put(1)
}

func Test_PoolMemLeak(t *testing.T) {
	p := Pool[int]{
		New: func() int { return 0 },
		Max: 1,
	}
	p.Put(1)
	p.Put(2)
	if len(p.pool) > 1 {
		t.Errorf("memory leaked")
	}
}

func Test_PoolExceedMaxNew(t *testing.T) {
	p := Pool[int]{
		New:    func() int { return 0 },
		Debug:  true,
		MaxNew: 2,
	}

	defer func() {
		if err := recover(); err == nil {
			t.Errorf("should panic")
		}
	}()
	p.Get()
	p.Get()
	p.Get()
}
