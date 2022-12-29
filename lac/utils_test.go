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
)

func TestNoMalloc(t *testing.T) {
	defer func() {
		if err := recover(); err == nil {
			t.Errorf("should panic")
		}
	}()
	var i *int
	noMalloc(func() {
		i = new(int)
	})
	runtime.KeepAlive(i)
}

func Test_NoEscape(t *testing.T) {
	s := []int{1, 2}
	m := map[int]int{1: 10, 2: 20}

	noMalloc(func() {
		i := 1
		_ = noEscape(i)
		_ = noEscape(s)
		_ = noEscape(m)
	})
}
