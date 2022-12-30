/*
 * Linear Allocator
 *
 * Improve the memory allocation and garbage collection performance.
 *
 * Copyright (C) 2020-2022 crazybie@github.com.
 * https://github.com/crazybie/linear_ac
 */

package gls

import (
	"math/rand"
	"testing"
	"time"
)

func Test_GoRoutineId(t *testing.T) {
	id := GoRoutineId()
	if id != goRoutineIdSlow() {
		t.Fail()
	}
}

func BenchmarkGoRoutineId(b *testing.B) {
	k := goRoutineIdSlow()
	for i := 0; i < b.N; i++ {
		if GoRoutineId() != k {
			b.Fail()
		}
	}
}

func BenchmarkGoRoutineIdSlow(b *testing.B) {
	k := goRoutineIdSlow()
	for i := 0; i < b.N; i++ {
		if goRoutineIdSlow() != k {
			b.Fail()
		}
	}
}

func Test_GlsSmoke(t *testing.T) {
	s := NewGls[string](func() string { return "0" })

	if s.Get() != "0" {
		t.Errorf("not zero")
	}

	s.Set("ab")
	if s.Get() != "ab" {
		t.Errorf("err get")
	}

	v := s.Get(WithValidateFn(func(t string) bool {
		return len(t) > 1
	}))
	if v != "ab" {
		t.Errorf("failed to get")
	}

	v = s.Get(WithValidateFn(func(t string) bool {
		return t == "abc"
	}))
	if v != "0" {
		t.Errorf("zero after validate faield")
	}

	v = s.Get(WithValidateFn(func(t string) bool {
		return t == "abc"
	}), WithCreateFn(func() string {
		return "abc"
	}))
	if v != "abc" {
		t.Errorf("failed to create")
	}
}

func Test_GlsCrossGoroutines(t *testing.T) {
	s := NewGls[int](nil)

	for i := 0; i < 1000; i++ {
		go func(i int) {
			time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)
			s.Set(i)
			time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)
			if s.Get() != i {
				t.Fail()
			}
		}(i)
	}
}
