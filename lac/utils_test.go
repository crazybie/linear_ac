/*
 * Linear Allocator
 *
 * Improve the memory allocation and garbage collection performance.
 *
 * Copyright (C) 2020-2022 crazybie@github.com.
 * https://github.com/crazybie/linear_ac
 */

package lac

import "testing"

func Test_GoRoutineId(t *testing.T) {
	id := goRoutineId()
	if id != goRoutineIdSlow() {
		t.Fail()
	}
}

func Test_NoEscape(t *testing.T) {
	s := []int{1, 2}
	m := map[int]int{1: 10, 2: 20}

	noMalloc(func() {
		i := 1
		_ = noEscape(i)
		_ = noEscape(s)
		_ = noEscape(m)
	})
}
