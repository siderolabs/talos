#!/toolchain/bin/bash

set -e
# set SELinux labels for files according to file_contexts supplied
/toolchain/sbin/setfiles -r $1 -F -vv $3 $1
mksquashfs $1 $2 -all-root -noappend -comp zstd -Xcompression-level $4 -no-progress
