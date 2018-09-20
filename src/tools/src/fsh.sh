#!/bin/bash

PREFIX="${1}"
set -e

rm -rf /bin

mkdir -pv ${PREFIX}/{dev,etc,lib,opt,proc,sys}

mkdir -pv ${PREFIX}/bin
ln -sv /bin $PREFIX/sbin
# Required by glibc (looks for interpreter here)
ln -sv /lib $PREFIX/lib64

mkdir -pv ${PREFIX}/usr/{include,}
ln -sv /bin ${PREFIX}/usr/bin
ln -sv /bin ${PREFIX}/usr/sbin
ln -sv /lib ${PREFIX}/usr/lib
ln -sv /var ${PREFIX}/usr/var

mkdir -pv ${PREFIX}/usr/local
ln -sv /bin ${PREFIX}/usr/local/bin
ln -sv /bin ${PREFIX}/usr/local/sbin
ln -sv /lib ${PREFIX}/usr/local/lib
ln -sv /usr/include ${PREFIX}/usr/local/include

mkdir -pv ${PREFIX}/run
mkdir -pv ${PREFIX}/var/{log,mail,spool}
ln -sv /run $PREFIX/var/run

install -dv -m 0750 ${PREFIX}/root
install -dv -m 1777 ${PREFIX}/tmp ${PREFIX}/var/tmp

for d in ${PREFIX}/*; do
    _d=/$(basename $d)
    if [[ ! -d $_d ]]; then
        echo $_d
        ln -sv $d $_d
    fi
done

# Required by Docker
ln -sv /tools/bin/bash /bin/sh
ln -sv /tools/bin/bash /bin/bash
