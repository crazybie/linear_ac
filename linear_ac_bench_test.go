package linear_ac

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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

var (
	totalTasks        = 100000
	totalGoroutines   = 1000
	busyGoroutineId   = 1
	busyGoroutineLoop = 500
)

func Schedule(genTasks func(chan func(int))) {
	wait := sync.WaitGroup{}
	wait.Add(totalGoroutines)

	queue := make(chan func(int), totalGoroutines)
	maxLatency, totalLatency := int64(0), int64(0)
	cnt := int64(0)

	for i := 0; i < totalGoroutines; i++ {
		go func(routineId int) {
			for f := range queue {

				s := time.Now().UnixNano()
				f(routineId)

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

	fmt.Printf("latency: max=%vms, avg=%vms.\n", maxLatency/1000/1000, totalLatency/cnt/1000/1000)
}

func Benchmark_LinearAc(t *testing.B) {
	DbgMode = false
	chunkPool.reserve(1600)
	runtime.GC()

	Schedule(func(queue chan func(int)) {
		for i := 0; i < totalTasks; i++ {
			queue <- func(routine int) {

				subLoop := 1
				if routine == busyGoroutineId {
					subLoop = busyGoroutineLoop
				}

				for n := 0; n < subLoop; n++ {
					ac := BindNew()

					makeItem := func(j int) *PbItemEx {
						return ac.NewCopy(&PbItemEx{
							Id1:     ac.Int(2 + j),
							Id2:     ac.Int(2 + j),
							Id3:     ac.Int(2 + j),
							Id4:     ac.Int(2 + j),
							Id5:     ac.Int(2 + j),
							Id6:     ac.Int(2 + j),
							Id7:     ac.Int(2 + j),
							Id8:     ac.Int(2 + j),
							Id9:     ac.Int(2 + j),
							Id10:    ac.Int(2 + j),
							Price:   ac.Int(100 + j),
							Class:   ac.Int(3 + j),
							Name1:   ac.String("name"),
							Active:  ac.Bool(true),
							EnumVal: ac.Enum(EnumVal2).(*EnumA),
						}).(*PbItemEx)
					}

					var d *PbDataEx
					ac.New(&d)
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

					n := 100
					for j := 0; j < n; j++ {
						ac.SliceAppend(&d.Items1, makeItem(j))
					}
					for j := 0; j < n; j++ {
						ac.SliceAppend(&d.Items2, makeItem(j))
					}
					for j := 0; j < n; j++ {
						ac.SliceAppend(&d.Items3, makeItem(j))
					}
					for j := 0; j < n; j++ {
						ac.SliceAppend(&d.Items4, makeItem(j))
					}
					for j := 0; j < n; j++ {
						ac.SliceAppend(&d.Items5, makeItem(j))
					}
					for j := 0; j < n; j++ {
						ac.SliceAppend(&d.Items6, makeItem(j))
					}
					for j := 0; j < n; j++ {
						ac.SliceAppend(&d.Items7, makeItem(j))
					}
					for j := 0; j < n; j++ {
						ac.SliceAppend(&d.Items8, makeItem(j))
					}
					for j := 0; j < n; j++ {
						ac.SliceAppend(&d.Items9, makeItem(j))
					}

					ac.Release()
				}
			}
		}
	})
}

func Benchmark_buildInAc(t *testing.B) {

	newInt := func(v int) *int { return &v }
	newStr := func(v string) *string { return &v }
	newBool := func(v bool) *bool { return &v }
	newEnum := func(v EnumA) *EnumA { return &v }

	acPool.clear()
	chunkPool.clear()

	runtime.GC()

	Schedule(func(c chan func(int)) {
		for i := 0; i < totalTasks; i++ {
			c <- func(routine int) {

				subLoop := 1
				if routine == busyGoroutineId {
					subLoop = busyGoroutineLoop
				}

				for n := 0; n < subLoop; n++ {

					makeItem := func(j int) *PbItemEx {
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

					n := 100
					for j := 0; j < n; j++ {
						d.Items1 = append(d.Items1, makeItem(j))
					}
					for j := 0; j < n; j++ {
						d.Items2 = append(d.Items2, makeItem(j))
					}
					for j := 0; j < n; j++ {
						d.Items3 = append(d.Items3, makeItem(j))
					}
					for j := 0; j < n; j++ {
						d.Items4 = append(d.Items4, makeItem(j))
					}
					for j := 0; j < n; j++ {
						d.Items5 = append(d.Items5, makeItem(j))
					}
					for j := 0; j < n; j++ {
						d.Items6 = append(d.Items6, makeItem(j))
					}
					for j := 0; j < n; j++ {
						d.Items7 = append(d.Items7, makeItem(j))
					}
					for j := 0; j < n; j++ {
						d.Items8 = append(d.Items8, makeItem(j))
					}
					for j := 0; j < n; j++ {
						d.Items9 = append(d.Items9, makeItem(j))
					}
				}
			}
		}
	})

}
