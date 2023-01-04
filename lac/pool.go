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

package lac

import (
	"fmt"
	"sync"
)

type Pool[T any] struct {
	m     sync.Mutex
	New   func() T
	pool  []T
	Max   int
	Debug bool
	Equal func(a, b T) bool
}

func (p *Pool[T]) Get() T {
	p.m.Lock()
	defer p.m.Unlock()

	if len(p.pool) == 0 {
		return p.New()
	}
	r := p.pool[len(p.pool)-1]
	p.pool = p.pool[:len(p.pool)-1]
	return r
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
		p.pool[i] = p.New()
	}
}
