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
	"flag"
	"fmt"
	"runtime"
	"testing"
)

type PbItemEx struct {
	Id1     *int
	Id2     *int
	Id3     *int
	Id4     *int
	Id5     *int
	Id6     *int
	Id7     *int
	Id8     *int
	Id9     *int
	Id10    *int
	Price   *int
	Class   *int
	Name1   *string
	Active  *bool
	EnumVal *EnumA
}

type PbDataEx struct {
	Age1    *int
	Age2    *int
	Age3    *int
	Age4    *int
	Age5    *int
	Age6    *int
	Age7    *int
	Age8    *int
	Age9    *int
	Age10   *int
	Items1  []*PbItemEx
	Items2  []*PbItemEx
	Items3  []*PbItemEx
	Items4  []*PbItemEx
	Items5  []*PbItemEx
	Items6  []*PbItemEx
	Items7  []*PbItemEx
	Items8  []*PbItemEx
	Items9  []*PbItemEx
	InUse1  *PbItemEx
	InUse2  *PbItemEx
	InUse3  *PbItemEx
	InUse4  *PbItemEx
	InUse5  *PbItemEx
	InUse6  *PbItemEx
	InUse7  *PbItemEx
	InUse8  *PbItemEx
	InUse9  *PbItemEx
	InUse10 *PbItemEx
}

var makeItemAc = func(j int, ac *Allocator) *PbItemEx {
	r := New[PbItemEx](ac)
	r.Id1 = ac.Int(2 + j)
	r.Id2 = ac.Int(2 + j)
	r.Id3 = ac.Int(2 + j)
	r.Id4 = ac.Int(2 + j)
	r.Id5 = ac.Int(2 + j)
	r.Id6 = ac.Int(2 + j)
	r.Id7 = ac.Int(2 + j)
	r.Id8 = ac.Int(2 + j)
	r.Id9 = ac.Int(2 + j)
	r.Id10 = ac.Int(2 + j)
	r.Price = ac.Int(100 + j)
	r.Class = ac.Int(3 + j)
	r.Name1 = ac.String("name")
	r.Active = ac.Bool(true)
	r.EnumVal = NewEnum(ac, EnumVal2)
	return r
}

var itemLoop = 10

func makeDataAc(ac *Allocator, i int) *PbDataEx {

	d := New[PbDataEx](ac)
	d.Age1 = ac.Int(11 + i)
	d.Age2 = ac.Int(11 + i)
	d.Age3 = ac.Int(11 + i)
	d.Age4 = ac.Int(11 + i)
	d.Age5 = ac.Int(11 + i)
	d.Age6 = ac.Int(11 + i)
	d.Age7 = ac.Int(11 + i)
	d.Age8 = ac.Int(11 + i)
	d.Age9 = ac.Int(11 + i)
	d.Age10 = ac.Int(11 + i)

	d.InUse1 = makeItemAc(i, ac)
	d.InUse2 = makeItemAc(i, ac)
	d.InUse3 = makeItemAc(i, ac)
	d.InUse4 = makeItemAc(i, ac)
	d.InUse5 = makeItemAc(i, ac)
	d.InUse6 = makeItemAc(i, ac)
	d.InUse7 = makeItemAc(i, ac)
	d.InUse8 = makeItemAc(i, ac)
	d.InUse9 = makeItemAc(i, ac)
	d.InUse10 = makeItemAc(i, ac)

	for j := 0; j < itemLoop; j++ {
		d.Items1 = Append(ac, d.Items1, makeItemAc(j, ac))
	}
	for j := 0; j < itemLoop; j++ {
		d.Items2 = Append(ac, d.Items2, makeItemAc(j, ac))
	}
	for j := 0; j < itemLoop; j++ {
		d.Items3 = Append(ac, d.Items3, makeItemAc(j, ac))
	}
	for j := 0; j < itemLoop; j++ {
		d.Items4 = Append(ac, d.Items4, makeItemAc(j, ac))
	}
	for j := 0; j < itemLoop; j++ {
		d.Items5 = Append(ac, d.Items5, makeItemAc(j, ac))
	}
	for j := 0; j < itemLoop; j++ {
		d.Items6 = Append(ac, d.Items6, makeItemAc(j, ac))
	}
	for j := 0; j < itemLoop; j++ {
		d.Items7 = Append(ac, d.Items7, makeItemAc(j, ac))
	}
	for j := 0; j < itemLoop; j++ {
		d.Items8 = Append(ac, d.Items8, makeItemAc(j, ac))
	}
	for j := 0; j < itemLoop; j++ {
		d.Items9 = Append(ac, d.Items9, makeItemAc(j, ac))
	}
	return d
}

func BenchmarkNew(b *testing.B) {
	ac := acPool.Get()
	defer ac.Release()

	var item *PbItem
	for i := 0; i < b.N; i++ {
		item = New[PbItem](ac)
		item.Id = ac.Int(i)
	}
	runtime.KeepAlive(item)
}

func BenchmarkNewFrom(b *testing.B) {
	ac := acPool.Get()
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

	t.ResetTimer()
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

var stats bool

func init() {
	flag.BoolVar(&stats, "stats", false, "")
}

func Benchmark_LacMalloc(t *testing.B) {
	acPool.EnableDebugMode(false)
	ac := acPool.Get()
	if stats {
		defer func() { fmt.Println(acPool.DumpStats(true)) }()
	}
	defer ac.Release()

	t.ResetTimer()
	var e *PbItem
	for i := 0; i < t.N; i++ {
		e = New[PbItem](ac)
		e.Name = ac.String("a")
		e.Class = ac.Int(i)
		e.Id = ac.Int(i + 10)
		e.Active = ac.Bool(true)
	}
	runtime.KeepAlive(e)
	t.StopTimer()
}

func Benchmark_LacMallocMt(t *testing.B) {
	acPool.EnableDebugMode(false)
	ac := acPool.Get()
	ac.IncRef()
	if stats {
		defer func() { fmt.Println(acPool.DumpStats(true)) }()
	}
	defer ac.Release()

	t.ResetTimer()
	var e *PbItem
	for i := 0; i < t.N; i++ {
		e = New[PbItem](ac)
		e.Name = ac.String("a")
		e.Class = ac.Int(i)
		e.Id = ac.Int(i + 10)
		e.Active = ac.Bool(true)
	}
	runtime.KeepAlive(e)
	t.StopTimer()
}

func Benchmark_RawMallocLarge2(t *testing.B) {
	t.ResetTimer()
	var e *PbDataEx
	for i := 0; i < t.N; i++ {
		e = makeDataAc(nil, i)
	}
	runtime.KeepAlive(e)
	t.StopTimer()
}

func Benchmark_LacMallocLarge2(t *testing.B) {
	acPool.EnableDebugMode(false)

	ac := acPool.Get()
	if stats {
		defer func() { fmt.Println(acPool.DumpStats(true)) }()
	}
	defer ac.Release()

	t.ResetTimer()
	var e *PbDataEx
	for i := 0; i < t.N; i++ {
		e = makeDataAc(ac, i)
	}
	runtime.KeepAlive(e)
	t.StopTimer()
}

func Benchmark_LacMallocLarge2Mt(t *testing.B) {
	acPool.EnableDebugMode(false)
	ac := acPool.Get()
	ac.IncRef()
	if stats {
		defer func() { fmt.Println(acPool.DumpStats(true)) }()
	}
	defer ac.Release()

	t.ResetTimer()
	var e *PbDataEx
	for i := 0; i < t.N; i++ {
		e = makeDataAc(ac, i)
	}
	runtime.KeepAlive(e)
	t.StopTimer()
}
