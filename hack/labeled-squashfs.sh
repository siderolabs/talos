#!/bin/bash

set -e
setfiles -r $1 -F -vv /file_contexts $1
mksquashfs $1 $2 -all-root -noappend -comp zstd -Xcompression-level $3 -no-progress
