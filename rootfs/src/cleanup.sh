#!/bin/bash

function remove_symlinks() {
  set +e
  for l in $(find /rootfs -type l); do
    readlink $l | grep -q /tools
    if [ $? == 0 ]; then
        echo "Unlinking $l"
        unlink $l
    fi
  done
  set -e
}

remove_symlinks
find /rootfs -type f -name \*.a -print0 | xargs -0 rm -rf
find /rootfs/lib /rootfs/usr/lib -type f \( -name \*.so* -a ! -name \*dbg \) -exec strip --strip-unneeded {} ';'
find /rootfs/{bin,sbin} /rootfs/usr/{bin,sbin,libexec} -type f -exec strip --strip-all {} ';'

rm -rf /rootfs/usr/include/*
rm -rf /rootfs/usr/share/*
