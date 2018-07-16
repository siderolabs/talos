#!/bin/bash

function remove_symlinks() {
  set +e
  for l in $(find ${PREFIX} -type l); do
    readlink $l | grep -q /tools
    if [ $? == 0 ]; then
        echo "Unlinking $l"
        unlink $l
    fi
  done
  set -e
}

PREFIX="${1}"

remove_symlinks
find ${PREFIX} -type f -name \*.a -print0 | xargs -0 rm -rf
find ${PREFIX}/lib ${PREFIX}/usr/lib -type f \( -name \*.so* -a ! -name \*dbg \) -exec strip --strip-unneeded {} ';'
find ${PREFIX}/{bin,sbin} ${PREFIX}/usr/{bin,sbin,libexec} -type f -exec strip --strip-all {} ';'

rm -rf ${PREFIX}/usr/include/*
rm -rf ${PREFIX}/usr/share/*

mkdir -p /usr/share
mkdir -p /usr/local/share

paths=( /etc/pki /usr/share/ca-certificates /usr/local/share/ca-certificates /etc/ca-certificates )
for d in "${paths[@]}"; do
  ln -sv /etc/ssl/certs ${PREFIX}$d
done
