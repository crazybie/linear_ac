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
	debugMode        = false
	DisableLac       = false
	MaxLac           = 10000     // Lacs exceed this threshold will not be returned to the runtime.
	MaxNewLacInDebug = 20        // detect whether user call Release or DecRef correctly in debug mode.
	ChunkSize        = 64 * 1024 // larger request use the system allocator.
	DefaultChunks    = 4 * 1024
	MaxChunks        = DefaultChunks * 2 // chunks exceed this threshold will be returned to the runtime.
)
