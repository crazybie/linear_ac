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

go test -run . -v -race || exit 1

echo "===================="
echo "Following tests must be FAILED."
echo "===================="
! SimulateCrash=1 go test -run TestSliceWbPanic -v || exit 1

echo "===================="
echo "All tests passed."
echo "===================="