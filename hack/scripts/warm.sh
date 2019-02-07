#!/bin/bash

set -e

TMP=$(mktemp -d)

trap "rm -rf ${TMP}" EXIT

SOURCE=$(pwd)/build/rootfs.tar.gz
DEST=$(pwd)/images/rootfs-warm.tar.gz

tar -xvpzf ${SOURCE} -C ${TMP}
cp -R images ${TMP}/usr
cp ./build/init ${TMP}
mount -v --bind /dev ${TMP}/dev
chroot ${TMP} /init --warm
umount -lv ${TMP}/dev
rm -rf ${TMP}/usr/images
rm ${TMP}/init
tar -cvpzf ${DEST} -C ${TMP} .

