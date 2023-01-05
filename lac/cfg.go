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
	debugMode  = false
	DisableLac = false
	ChunkSize  = 1024 * 256 // larger request use the system allocator.
	MaxChunks  = 2000       // chunks exceed this threshold will be returned the runtime.
	MaxLac     = 10000      // Lacs exceed this threshold will not be returned the runtime.
)
