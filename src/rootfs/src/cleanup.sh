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

rm -rf \
  ${PREFIX}/bin/getconf \
  ${PREFIX}/bin/ldd \
  ${PREFIX}/bin/mtrace \
  ${PREFIX}/bin/gencat \
  ${PREFIX}/bin/locale \
  ${PREFIX}/bin/xtrace \
  ${PREFIX}/bin/zic \
  ${PREFIX}/bin/sln \
  ${PREFIX}/bin/tzselect \
  ${PREFIX}/bin/iconv \
  ${PREFIX}/bin/sotruss \
  ${PREFIX}/bin/ldconfig \
  ${PREFIX}/bin/pldd \
  ${PREFIX}/bin/iconvconfig \
  ${PREFIX}/bin/localedef \
  ${PREFIX}/bin/makedb \
  ${PREFIX}/bin/pcprofiledump \
  ${PREFIX}/bin/nscd \
  ${PREFIX}/bin/sprof \
  ${PREFIX}/bin/zdump \
  ${PREFIX}/bin/getent \
  ${PREFIX}/bin/scmp_sys_resolver \
  ${PREFIX}/bin/catchsegv \
  ${PREFIX}/lib/gconv/ \
  ${PREFIX}/usr/include/* \
  ${PREFIX}/usr/share/* \
  ${PREFIX}/usr/libexec/getconf

mkdir -p /usr/share
mkdir -p /usr/local/share

paths=( /etc/pki /usr/share/ca-certificates /usr/local/share/ca-certificates /etc/ca-certificates )
for d in "${paths[@]}"; do
  ln -sv /etc/ssl/certs ${PREFIX}$d
done
