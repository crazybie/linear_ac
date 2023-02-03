/*
 * Linear Allocator
 *
 * Improve the memory allocation and garbage collection performance.
 *
 * Copyright (C) 2020-2023 crazybie@github.com.
 * https://github.com/crazybie/linear_ac
 */

package lac

const (
	ChunkSize     = 128 * 1024 // larger request use the system allocator.
	DefaultChunks = 4 * 1024   // 512M
)

var (
	debugMode        = false
	DisableLac       = false
	MaxLac           = 10000             // Lacs exceed this threshold will not be returned to the runtime.
	MaxNewLacInDebug = 20                // detect whether user call Release or DecRef correctly in debug mode.
	MaxChunks        = DefaultChunks * 2 // chunks exceed this threshold will be returned to the runtime.

	// our memory is much cheaper than systems,
	// so we can be more aggressive than `append`.
	SliceExtendRatio float64 = 2.5
)
