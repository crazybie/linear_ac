//go:build goexperiment.arenas

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
	"arena"
	"runtime"
	"testing"
)

var makeItemArena = func(j int, ac *arena.Arena) *PbItemEx {
	e := arena.New[PbItemEx](ac)
	e.Id1 = arena.New[int](ac)
	*e.Id1 = 2 + j
	e.Id2 = arena.New[int](ac)
	*e.Id2 = 2 + j
	e.Id3 = arena.New[int](ac)
	*e.Id3 = 2 + j
	e.Id4 = arena.New[int](ac)
	*e.Id4 = 2 + j
	e.Id5 = arena.New[int](ac)
	*e.Id5 = 2 + j
	e.Id6 = arena.New[int](ac)
	*e.Id6 = 2 + j
	e.Id7 = arena.New[int](ac)
	*e.Id7 = 2 + j
	e.Id8 = arena.New[int](ac)
	*e.Id8 = 2 + j
	e.Id9 = arena.New[int](ac)
	*e.Id9 = 2 + j
	e.Id10 = arena.New[int](ac)
	*e.Id10 = 2 + j
	e.Price = arena.New[int](ac)
	*e.Price = 100 + j
	e.Class = arena.New[int](ac)
	*e.Class = 3 + j
	e.Name1 = arena.New[string](ac)
	*e.Name1 = "name"
	e.Active = arena.New[bool](ac)
	*e.Active = true
	e.EnumVal = arena.New[EnumA](ac)
	*e.EnumVal = EnumVal2
	return e
}

func makeDataArena(ac *arena.Arena, i int) *PbDataEx {
	d := arena.New[PbDataEx](ac)
	d.Age1 = arena.New[int](ac)
	*d.Age1 = 11 + i
	d.Age2 = arena.New[int](ac)
	*d.Age2 = 11 + i
	d.Age3 = arena.New[int](ac)
	*d.Age3 = 11 + i
	d.Age4 = arena.New[int](ac)
	*d.Age4 = 11 + i
	d.Age5 = arena.New[int](ac)
	*d.Age5 = 11 + i
	d.Age6 = arena.New[int](ac)
	*d.Age6 = 11 + i
	d.Age7 = arena.New[int](ac)
	*d.Age7 = 11 + i
	d.Age8 = arena.New[int](ac)
	*d.Age8 = 11 + i
	d.Age9 = arena.New[int](ac)
	*d.Age9 = 11 + i
	d.Age10 = arena.New[int](ac)
	*d.Age10 = 11 + i

	d.InUse1 = makeItemArena(i, ac)
	d.InUse2 = makeItemArena(i, ac)
	d.InUse3 = makeItemArena(i, ac)
	d.InUse4 = makeItemArena(i, ac)
	d.InUse5 = makeItemArena(i, ac)
	d.InUse6 = makeItemArena(i, ac)
	d.InUse7 = makeItemArena(i, ac)
	d.InUse8 = makeItemArena(i, ac)
	d.InUse9 = makeItemArena(i, ac)
	d.InUse10 = makeItemArena(i, ac)

	d.Items1 = arena.MakeSlice[*PbItemEx](ac, 0, 0)
	for j := 0; j < itemLoop; j++ {
		d.Items1 = append(d.Items1, makeItemArena(j, ac))
	}
	d.Items2 = arena.MakeSlice[*PbItemEx](ac, 0, 0)
	for j := 0; j < itemLoop; j++ {
		d.Items2 = append(d.Items2, makeItemArena(j, ac))
	}
	d.Items3 = arena.MakeSlice[*PbItemEx](ac, 0, 0)
	for j := 0; j < itemLoop; j++ {
		d.Items3 = append(d.Items3, makeItemArena(j, ac))
	}
	d.Items4 = arena.MakeSlice[*PbItemEx](ac, 0, 0)
	for j := 0; j < itemLoop; j++ {
		d.Items4 = append(d.Items4, makeItemArena(j, ac))
	}
	d.Items5 = arena.MakeSlice[*PbItemEx](ac, 0, 0)
	for j := 0; j < itemLoop; j++ {
		d.Items5 = append(d.Items5, makeItemArena(j, ac))
	}
	d.Items6 = arena.MakeSlice[*PbItemEx](ac, 0, 0)
	for j := 0; j < itemLoop; j++ {
		d.Items6 = append(d.Items6, makeItemArena(j, ac))
	}
	d.Items7 = arena.MakeSlice[*PbItemEx](ac, 0, 0)
	for j := 0; j < itemLoop; j++ {
		d.Items7 = append(d.Items7, makeItemArena(j, ac))
	}
	d.Items8 = arena.MakeSlice[*PbItemEx](ac, 0, 0)
	for j := 0; j < itemLoop; j++ {
		d.Items8 = append(d.Items8, makeItemArena(j, ac))
	}
	d.Items9 = arena.MakeSlice[*PbItemEx](ac, 0, 0)
	for j := 0; j < itemLoop; j++ {
		d.Items9 = append(d.Items9, makeItemArena(j, ac))
	}
	return d
}

func Benchmark_LacMallocLarge(t *testing.B) {
	DbgMode = false
	runtime.GC()
	ac := Get()

	t.StartTimer()
	for i := 0; i < t.N; i++ {
		_ = makeDataAc(ac, i)
	}
	t.StartTimer()

	ac.Release()
	acPool.clear()
	chunkPool.clear()
}

func Benchmark_ArenaMallocLarge(t *testing.B) {
	runtime.GC()
	ac := arena.NewArena()
	t.StartTimer()
	for i := 0; i < t.N; i++ {
		_ = makeDataArena(ac, i)
	}
	t.StopTimer()
	ac.Free()
}

func Benchmark_LacMallocSmall(t *testing.B) {
	DbgMode = false
	runtime.GC()
	ac := Get()

	t.StartTimer()
	for i := 0; i < t.N; i++ {
		e := New[PbItem](ac)
		e.Name = ac.String("a")
		e.Class = ac.Int(1)
		e.Id = ac.Int(2)
		e.Active = ac.Bool(true)
	}
	t.StopTimer()
	ac.Release()
}

func Benchmark_ArenaMallocSmall(t *testing.B) {
	runtime.GC()
	ac := arena.NewArena()

	t.StartTimer()
	for i := 0; i < t.N; i++ {
		e := arena.New[PbItem](ac)
		e.Name = arena.New[string](ac)
		*e.Name = "a"
		e.Class = arena.New[int](ac)
		*e.Class = 1
		e.Id = arena.New[int](ac)
		*e.Id = 2
		e.Active = arena.New[bool](ac)
		*e.Active = true
	}
	t.StopTimer()

	ac.Free()
}
