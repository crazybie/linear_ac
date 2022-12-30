/*
 * Linear Allocator
 *
 * Improve the memory allocation and garbage collection performance.
 *
 * Copyright (C) 2020-2022 crazybie@github.com.
 * https://github.com/crazybie/linear_ac
 */

// Pool pros over sync.Pool:
// 1. generic API
// 2. no boxing
// 3. support reserving.

package lac

import "sync"

type Pool[T any] struct {
	m    sync.Mutex
	New  func() T
	pool []T
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
	p.pool = append(p.pool, v)
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
