#!/bin/sh

set -e

function iso() {
  mkdir -p ./boot/isolinux
  cp /usr/local/src/syslinux/bios/core/isolinux.bin ./boot/isolinux/isolinux.bin
  cp /usr/local/src/syslinux/bios/com32/elflink/ldlinux/ldlinux.c32 ./boot/isolinux/ldlinux.c32
  cat <<EOF >./boot/isolinux/isolinux.cfg
DEFAULT Dianemo
  SAY Dianemo
LABEL Dianemo
  KERNEL /boot/vmlinuz
  INITRD /boot/initramfs.xz
  APPEND quiet ip=dhcp console=tty1 console=ttyS0 dianemo.autonomy.io/root=/dev/sda
EOF
  mkisofs -o /out/dianemo.iso -b boot/isolinux/isolinux.bin -c boot/isolinux/boot.cat -no-emul-boot -boot-load-size 4 -boot-info-table .
}

function raw() {
  dd if=/dev/zero of=/dianemo.raw bs=1M count=2000
  parted -s /dianemo.raw mklabel gpt
  parted -s -a none /dianemo.raw mkpart ESP fat32 0 50M
  parted -s -a none /dianemo.raw mkpart ROOT xfs 50M 750M
  parted -s -a none /dianemo.raw mkpart DATA xfs 750M 2G
  losetup /dev/loop0 /dianemo.raw
  partx -av /dev/loop0
  sgdisk /dev/loop0 --attributes=1:set:2
  mkfs.vfat /dev/loop0p1
  mkfs.xfs -L ROOT /dev/loop0p2
  mkfs.xfs -L DATA /dev/loop0p3
  mount /dev/loop0p1 /mnt
  mkdir -p /mnt/boot/extlinux
  extlinux --install /mnt/boot/extlinux
  cat <<EOF >/mnt/boot/extlinux/extlinux.conf
DEFAULT Dianemo
  SAY Dianemo by Autonomy
LABEL Dianemo
  KERNEL /boot/vmlinuz
  INITRD /boot/initramfs.xz
  APPEND quiet ip=dhcp console=tty1 console=ttyS0 dianemo.autonomy.io/root=/dev/sda
EOF
  cp -v /rootfs/boot/* /mnt/boot
  umount /mnt
  mount /dev/loop0p2 /mnt
  cp -Rv ./* /mnt
  rm -rf /mnt/boot
  rm -rf /mnt/var/*
  umount /mnt
  mount /dev/loop0p3 /mnt
  cp -Rv ./var/* /mnt
  dd if=/usr/local/src/syslinux/efi64/mbr/gptmbr.bin of=/dev/loop0
  cleanup
  cp -v /dianemo.raw /out
  qemu-img convert -f raw -O vmdk /out/dianemo.raw /out/dianemo.vmdk
}

function rootfs() {
  dd if=/dev/zero of=/rootfs.raw bs=1M count=750
  parted -s /rootfs.raw mklabel gpt
  parted -s -a none /rootfs.raw mkpart ROOT xfs 0 700M
  parted -s -a none /rootfs.raw mkpart DATA xfs 700M 750M
  losetup /dev/loop0 /rootfs.raw
  partx -av /dev/loop0
  mkfs.xfs -L ROOT /dev/loop0p1
  mkfs.xfs -L DATA /dev/loop0p2
  mount /dev/loop0p1 /mnt
  cp -Rv ./* /mnt
  rm -rf /mnt/boot
  rm -rf /mnt/var/*
  umount /mnt
  mount /dev/loop0p2 /mnt
  cp -Rv ./var/* /mnt
  cleanup
  cp -v ./boot/vmlinuz /out
  cp -v ./boot/initramfs.xz /out
  cp -v /rootfs.raw /out
}

function cleanup {
  umount /mnt || true
  partx -dv /dev/loop0 || true
  losetup -d /dev/loop0 || true
}

trap cleanup EXIT

case "$1" in
        raw)
            rootfs
            raw
            iso
            ;;
        iso)
            iso
            ;;
        *)
            exec "$@"
esac
