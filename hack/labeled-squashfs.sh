#!/bin/bash

set -e
setfiles -r $1 -F -vv /file_contexts $1 | tee /rootfs/etc/selinux/talos/setfiles.log
mksquashfs $1 $2 -all-root -noappend -comp zstd -Xcompression-level $3 -no-progress
