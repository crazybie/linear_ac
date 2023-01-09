/*
 * Linear Allocator
 *
 * Improve the memory allocation and garbage collection performance.
 *
 * Copyright (C) 2020-2023 crazybie@github.com.
 * https://github.com/crazybie/linear_ac
 */

// Pool pros over sync.Pool:
// 1. generic API
// 2. no boxing
// 3. support reserving.
// 4. debug mode: leak detecting, duplicated put.
// 5. max size: memory leak protection.

package lac

import (
	"fmt"
)

type Pool[T any] struct {
	m      SpinLock
	New    func() T
	pool   []T
	Max    int
	newCnt int

	Debug  bool
	MaxNew int               // require Debug=true
	Equal  func(a, b T) bool // require Debug=true
}

func (p *Pool[T]) Get() T {
	p.m.Lock()
	defer p.m.Unlock()

	if len(p.pool) == 0 {
		return p.doNew()
	}
	r := p.pool[len(p.pool)-1]
	p.pool = p.pool[:len(p.pool)-1]
	return r
}

func (p *Pool[T]) doNew() T {
	p.newCnt++
	if p.Debug && p.MaxNew > 0 && p.newCnt > p.MaxNew {
		panic("creates too many objects, potential memory leak?")
	}
	return p.New()
}

func (p *Pool[T]) Put(v T) {
	p.m.Lock()
	defer p.m.Unlock()

	if p.Debug && p.Equal != nil {
		for _, i := range p.pool {
			if p.Equal(i, v) {
				panic(fmt.Errorf("already in pool: %v, %v", i, v))
			}
		}
	}

	if p.Max == 0 || len(p.pool) < p.Max {
		p.pool = append(p.pool, v)
	}
}

func (p *Pool[T]) Clear() {
	p.m.Lock()
	defer p.m.Unlock()

	p.pool = nil
}

func (p *Pool[T]) Reserve(cnt int) {
	p.m.Lock()
	defer p.m.Unlock()

	p.pool = make([]T, cnt)
	for i := 0; i < cnt; i++ {
		p.pool[i] = p.doNew()
	}
}
