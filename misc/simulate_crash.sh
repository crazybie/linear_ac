#!/bin/bash
#
# Linear Allocator
#
# Improve the memory allocation and garbage collection performance.
#
# Copyright (C) 2020-2023 crazybie@github.com.
# https://github.com/crazybie/linear_ac
#

cd "$(dirname "$0")/../lac"

go test -run TestSliceWbPanic || exit 1
! SimulateCrash=1 go test -run TestSliceWbPanic || exit 1