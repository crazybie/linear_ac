#!/bin/bash

cd "$(dirname "$0")/../lac"

go test -run . -v -race || exit 1

echo "===================="
echo "Following tests must be FAILED."
echo "===================="
! SimulateCrash=1 go test -run TestSliceWbPanic -v || exit 1
! SimulateCrash=1 go test -run TestFreeMarkedObj -v || exit 1

echo "===================="
echo "All tests passed."
echo "===================="