#!/bin/bash

function remove_symlinks() {
  set +e
  for l in $(find ${PREFIX} -type l); do
    readlink $l | grep -q /toolchain
    if [ $? == 0 ]; then
        echo "Unlinking $l"
        unlink $l
    fi
  done
  set -e
}

PREFIX="${1}"

remove_symlinks
find ${PREFIX} -type f -name \*.a -print0 | xargs -0 rm -rf || true
find ${PREFIX} -type f -name \*.la -print0 | xargs -0 rm -rf || true
find ${PREFIX}/lib ${PREFIX}/usr/lib -type f \( -name \*.so* -a ! -name \*dbg \) -exec strip --strip-unneeded {} ';' || true
find ${PREFIX}/{bin,sbin} -type f -exec strip --strip-all {} ';' || true

rm -rf \
  ${PREFIX}/lib/gconv/ \
  ${PREFIX}/lib/pkgconfig/ \
  ${PREFIX}/include/* \
  ${PREFIX}/share/* \
  ${PREFIX}/usr/include/* \
  ${PREFIX}/usr/share/* \
  ${PREFIX}/usr/libexec/getconf \
  ${PREFIX}/var/db
