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
	debugMode     = false
	DisableLac    = false
	ChunkSize     = 1 * 1024 * 1024 // larger request use the system allocator.
	MaxChunks     = 2000            // chunks exceed this threshold will be returned to the runtime.
	MaxLac        = 10000           // Lacs exceed this threshold will not be returned to the runtime.
	DefaultChunks = 64
)
