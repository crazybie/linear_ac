/*
 * Linear Allocator
 *
 * Improve the memory allocation and garbage collection performance.
 *
 * Copyright (C) 2020-2023 crazybie@github.com.
 * https://github.com/crazybie/linear_ac
 */

package lac

var (
	DisableAllLac    = false
	MaxNewLacInDebug = 200 // detect whether user call Release or DecRef correctly in debug mode.

	// our memory is much cheaper than systems,
	// so we can be more aggressive than `append`.
	SliceExtendRatio float64 = 2.5
)
