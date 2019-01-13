#!/bin/bash

set -e

PREFIX="${1}"

mkdir -pv ${PREFIX}/{dev,lib,proc,sys}

mkdir -pv ${PREFIX}/bin
ln -sv bin $PREFIX/sbin
ln -sv lib $PREFIX/lib64

mkdir -pv ${PREFIX}/usr/{include,}
ln -sv ../bin ${PREFIX}/usr/bin
ln -sv ../bin ${PREFIX}/usr/sbin
ln -sv ../lib ${PREFIX}/usr/lib
ln -sv ../var ${PREFIX}/usr/var

mkdir -pv ${PREFIX}/usr/local
ln -sv ../../bin ${PREFIX}/usr/local/bin
ln -sv ../../bin ${PREFIX}/usr/local/sbin
ln -sv ../../lib ${PREFIX}/usr/local/lib
ln -sv ../../usr/include ${PREFIX}/usr/local/include

mkdir -pv ${PREFIX}/run
mkdir -pv ${PREFIX}/var/{log,mail,spool}
ln -sv ../run $PREFIX/var/run

mkdir -p ${PREFIX}/var/etc
ln -sv var/etc ${PREFIX}/etc

mkdir -p ${PREFIX}/var/opt
ln -sv var/opt ${PREFIX}/opt

install -dv -m 0750 ${PREFIX}/root
install -dv -m 1777 ${PREFIX}/tmp ${PREFIX}/var/tmp
