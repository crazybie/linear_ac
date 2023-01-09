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
	"runtime"
	"testing"
)

func BenchmarkNew(b *testing.B) {
	ac := Get()
	defer ac.Release()

	var item *PbItem
	for i := 0; i < b.N; i++ {
		item = New[PbItem](ac)
		item.Id = ac.Int(i)
	}
	runtime.KeepAlive(item)
}

func BenchmarkNewFrom(b *testing.B) {
	ac := Get()
	defer ac.Release()

	var item *PbItem
	for i := 0; i < b.N; i++ {
		item = NewFrom(ac, &PbItem{
			Id: ac.Int(i),
		})
	}
	runtime.KeepAlive(item)
}
