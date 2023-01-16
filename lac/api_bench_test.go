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

func Benchmark_RawMalloc(t *testing.B) {
	runtime.GC()

	t.StartTimer()
	var e *PbItem
	for i := 0; i < t.N; i++ {
		e = new(PbItem)
		e.Name = new(string)
		*e.Name = "a"
		e.Class = new(int)
		*e.Class = i
		e.Id = new(int)
		*e.Id = i + 10
		e.Active = new(bool)
		*e.Active = true
	}
	runtime.KeepAlive(e)
	t.StopTimer()
}

func Benchmark_LacMalloc(t *testing.B) {
	EnableDebugMode(false)
	ReserveChunkPool(0)
	runtime.GC()
	ac := Get()
	defer ac.Release()

	t.StartTimer()
	var e *PbItem
	for i := 0; i < t.N; i++ {
		e = NewFrom(ac, &PbItem{
			Name:   ac.String("a"),
			Class:  ac.Int(i),
			Id:     ac.Int(i + 10),
			Active: ac.Bool(true),
		})
	}
	runtime.KeepAlive(e)
	t.StopTimer()
}

func Benchmark_RawMallocLarge2(t *testing.B) {
	runtime.GC()
	t.StartTimer()
	for i := 0; i < t.N; i++ {
		e := makeDataAc(nil, i)
		runtime.KeepAlive(e)
	}
	t.StopTimer()
}

func Benchmark_LacMallocLarge2(t *testing.B) {
	EnableDebugMode(false)
	ReserveChunkPool(0)
	runtime.GC()
	ac := Get()

	t.StartTimer()
	for i := 0; i < t.N; i++ {
		e := makeDataAc(ac, i)
		runtime.KeepAlive(e)
	}
	t.StartTimer()

	ac.Release()
	acPool.Clear()
	chunkPool.Clear()
}
