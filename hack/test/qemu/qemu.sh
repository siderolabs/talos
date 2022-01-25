#!/usr/bin/env bash

set -e

: ${TALOS_QEMU_ROOT:="/tmp"}

case $(uname -s) in
  Linux*)
    ACCEL=kvm
    ;;

  Darwin*)
    ACCEL=hvf
    ;;
  *)
    exit 1
    ;;
esac

KERNEL="_out/vmlinuz-amd64"
INITRD="_out/initramfs-amd64.xz"
IMAGE="$TALOS_QEMU_ROOT/rootfs.qcow2"
ISO="$TALOS_QEMU_ROOT/iso/config.iso"

talosctl gen config -o ${TALOS_QEMU_ROOT}/iso qemu https://10.254.0.10
cp ${TALOS_QEMU_ROOT}/iso/init.yaml ${TALOS_QEMU_ROOT}/iso/config.yaml
mkisofs -joliet -rock -volid 'metal-iso' -output ${ISO} ${TALOS_QEMU_ROOT}/iso
qemu-img create -f qcow2 ${IMAGE} 8G

qemu-system-x86_64 \
    -m 2048 \
    -accel ${ACCEL} \
    -cpu max \
    -smp 2 \
    -hda ${IMAGE} \
    -netdev user,id=talos,ipv4=on,net=10.254.0.0/24,dhcpstart=10.254.0.10,hostfwd=tcp::50000-:50000,hostfwd=tcp::6443-:6443,hostname=master-1 \
    -device virtio-net,netdev=talos \
    -nographic \
    -serial mon:stdio \
    -cdrom ${ISO} \
    -append "talos.platform=metal init_on_alloc=1 slab_nomerge pti=on printk.devkmsg=on earlyprintk=serial,tty0,keep console=tty0 talos.config=metal-iso" \
    -kernel ${KERNEL} \
    -initrd ${INITRD}
