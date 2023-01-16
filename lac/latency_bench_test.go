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
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var (
	totalTasks        = 100000
	totalGoroutines   = 500
	busyGoroutineId   = 1
	busyGoroutineLoop = 1000
)

var largeConfigData []*PbDataEx

func makeGlobalData() {
	if largeConfigData == nil {
		largeConfigData = make([]*PbDataEx, 10000)
		for i := range largeConfigData {
			largeConfigData[i] = makeData(i)
		}
	}
}

func Dispatch(genTasks func(chan func(int))) {
	wait := sync.WaitGroup{}
	wait.Add(totalGoroutines)
	queue := make(chan func(int), totalGoroutines)
	maxLatency, totalLatency := int64(0), int64(0)
	cnt := int64(0)

	for i := 0; i < totalGoroutines; i++ {
		go func(routineId int) {
			for task := range queue {

				s := time.Now().UnixNano()
				task(routineId)
				elapsed := time.Now().UnixNano() - s

				atomic.AddInt64(&totalLatency, elapsed)
				atomic.AddInt64(&cnt, 1)
				if elapsed > atomic.LoadInt64(&maxLatency) {
					atomic.StoreInt64(&maxLatency, elapsed)
				}
			}
			wait.Done()
		}(i)
	}

	genTasks(queue)
	close(queue)
	wait.Wait()

	fmt.Printf(">> Latency: max=%vms, avg=%vms.\n", maxLatency/1000/1000, totalLatency/cnt/1000/1000)
}

func BenchmarkLatencyLac(t *testing.B) {
	EnableDebugMode(false)
	ReserveChunkPool(256)
	makeGlobalData()
	runtime.GC()
	t.StartTimer()

	Dispatch(func(queue chan func(int)) {
		for i := 0; i < totalTasks; i++ {
			queue <- func(routine int) {

				subLoop := 1
				if routine == busyGoroutineId {
					subLoop = busyGoroutineLoop
				}

				for n := 0; n < subLoop; n++ {
					ac := Get()
					_ = makeDataAc(ac, n)
					ac.Release()
				}
			}
		}
	})

	acPool.Clear()
	chunkPool.Clear()
}

func BenchmarkLatencyRaw(t *testing.B) {
	makeGlobalData()
	runtime.GC()
	t.StartTimer()

	Dispatch(func(c chan func(int)) {
		for i := 0; i < totalTasks; i++ {
			c <- func(routine int) {

				subLoop := 1
				if routine == busyGoroutineId {
					subLoop = busyGoroutineLoop
				}

				for n := 0; n < subLoop; n++ {
					_ = makeData(n)
				}
			}
		}
	})
}

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

var newInt = func(v int) *int { return &v }
var newStr = func(v string) *string { return &v }
var newBool = func(v bool) *bool { return &v }
var newEnum = func(v EnumA) *EnumA { return &v }

var makeItem = func(j int) *PbItemEx {
	item := &PbItemEx{
		Id1:     newInt(2 + j),
		Id2:     newInt(2 + j),
		Id3:     newInt(2 + j),
		Id4:     newInt(2 + j),
		Id5:     newInt(2 + j),
		Id6:     newInt(2 + j),
		Id7:     newInt(2 + j),
		Id8:     newInt(2 + j),
		Id9:     newInt(2 + j),
		Id10:    newInt(2 + j),
		Price:   newInt(100 + j),
		Class:   newInt(3 + j),
		Name1:   newStr("name"),
		Active:  newBool(true),
		EnumVal: newEnum(EnumVal2),
	}
	return item
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

func makeData(i int) *PbDataEx {
	d := new(PbDataEx)
	d.Age1 = newInt(11 + i)
	d.Age2 = newInt(11 + i)
	d.Age3 = newInt(11 + i)
	d.Age4 = newInt(11 + i)
	d.Age5 = newInt(11 + i)
	d.Age6 = newInt(11 + i)
	d.Age7 = newInt(11 + i)
	d.Age8 = newInt(11 + i)
	d.Age9 = newInt(11 + i)
	d.Age10 = newInt(11 + i)

	d.InUse1 = makeItem(i)
	d.InUse2 = makeItem(i)
	d.InUse3 = makeItem(i)
	d.InUse4 = makeItem(i)
	d.InUse5 = makeItem(i)
	d.InUse6 = makeItem(i)
	d.InUse7 = makeItem(i)
	d.InUse8 = makeItem(i)
	d.InUse9 = makeItem(i)
	d.InUse10 = makeItem(i)

	for j := 0; j < itemLoop; j++ {
		d.Items1 = append(d.Items1, makeItem(j))
	}
	for j := 0; j < itemLoop; j++ {
		d.Items2 = append(d.Items2, makeItem(j))
	}
	for j := 0; j < itemLoop; j++ {
		d.Items3 = append(d.Items3, makeItem(j))
	}
	for j := 0; j < itemLoop; j++ {
		d.Items4 = append(d.Items4, makeItem(j))
	}
	for j := 0; j < itemLoop; j++ {
		d.Items5 = append(d.Items5, makeItem(j))
	}
	for j := 0; j < itemLoop; j++ {
		d.Items6 = append(d.Items6, makeItem(j))
	}
	for j := 0; j < itemLoop; j++ {
		d.Items7 = append(d.Items7, makeItem(j))
	}
	for j := 0; j < itemLoop; j++ {
		d.Items8 = append(d.Items8, makeItem(j))
	}
	for j := 0; j < itemLoop; j++ {
		d.Items9 = append(d.Items9, makeItem(j))
	}
	return d
}

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
