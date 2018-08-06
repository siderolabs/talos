#!/bin/bash

set -Eeuo pipefail

function create_image() {
  if [ "$RAW" = true ] ; then
    parted -s ${RAW_IMAGE} mklabel gpt
  else
    parted -s ${DEVICE} mklabel gpt
  fi

  if [ "$FULL" = true ] ; then
    if [ "$RAW" = true ] ; then
      parted -s -a optimal ${RAW_IMAGE} mkpart ESP fat32 0 50M
      parted -s -a optimal ${RAW_IMAGE} mkpart ROOT xfs 50M $(($(size) + 100))M
      parted -s -a optimal ${RAW_IMAGE} mkpart DATA xfs $(($(size) + 100))M 100%
      losetup ${DEVICE} ${RAW_IMAGE}
      partx -av ${DEVICE}
      extract_boot_partition ${DEVICE}p1
      extract_root_partition ${DEVICE}p2
      extract_data_partition ${DEVICE}p3
    else
      parted -s -a optimal ${DEVICE} mkpart ESP fat32 0 50M
      parted -s -a optimal ${DEVICE} mkpart ROOT xfs 50M $(($(size) + 100))M
      parted -s -a optimal ${DEVICE} mkpart DATA xfs $(($(size) + 100))M 100%
      extract_boot_partition ${DEVICE}1
      extract_root_partition ${DEVICE}2
      extract_data_partition ${DEVICE}3
    fi
  else
    if [ "$RAW" = true ] ; then
      parted -s -a optimal ${RAW_IMAGE} mkpart ROOT xfs 0 $(($(size) + 50))M
      parted -s -a optimal ${RAW_IMAGE} mkpart DATA xfs $(($(size) + 50))M 100%
      losetup ${DEVICE} ${RAW_IMAGE}
      partx -av ${DEVICE}
      extract_root_partition ${DEVICE}p1
      extract_data_partition ${DEVICE}p2
    else
      parted -s -a optimal ${DEVICE} mkpart ROOT xfs 0 $(($(size) + 50))M
      parted -s -a optimal ${DEVICE} mkpart DATA xfs $(($(size) + 50))M 100%
      extract_root_partition ${DEVICE}1
      extract_data_partition ${DEVICE}2
    fi
  fi

  sgdisk ${DEVICE} --attributes=1:set:2

  dd if=/usr/local/src/syslinux/efi64/mbr/gptmbr.bin of=${DEVICE}

  if [ "$RAW" = true ] ; then
    cleanup
  fi
}

function create_vmdk() {
  qemu-img convert -f raw -O vmdk ${RAW_IMAGE} ${VMDK_IMAGE}
}

function create_iso() {
  mkdir -p /mnt/boot/isolinux
  cp /usr/local/src/syslinux/bios/core/isolinux.bin /mnt/boot/isolinux/isolinux.bin
  cp /usr/local/src/syslinux/bios/com32/elflink/ldlinux/ldlinux.c32 /mnt/boot/isolinux/ldlinux.c32
  create_extlinux_conf /mnt/boot/isolinux/isolinux.conf
  tar -xpvJf /generated/rootfs.tar.xz -C /mnt
  mkisofs -o ${ISO_IMAGE} -b boot/isolinux/isolinux.bin -c boot/isolinux/boot.cat -no-emul-boot -boot-load-size 4 -boot-info-table .
}

function create_ami() {
  packer build -var "version=${VERSION}" "${@}" /packer.json
}

function size() {
  xz --robot --list /generated/rootfs.tar.xz | sed -n '3p' | cut -d$'\t' -f5 | awk '{printf("%.0f", $1*0.000001)}'
}

function extract_boot_partition() {
  local partition=$1
  mkfs.vfat ${partition}
  mount -v ${partition} /mnt
  mkdir -pv /mnt/boot/extlinux
  extlinux --install /mnt/boot/extlinux
  create_extlinux_conf /mnt/boot/extlinux/extlinux.conf
  cp /generated/boot/vmlinuz /mnt/boot
  cp /generated/boot/initramfs.xz /mnt/boot
  umount -v /mnt
}

function extract_root_partition() {
  local partition=$1
  mkfs.xfs -f -n ftype=1 -L ROOT ${partition}
  mount -v ${partition} /mnt
  tar -xpvJf /generated/rootfs.tar.xz --exclude="./var" -C /mnt
  umount -v /mnt
}

function extract_data_partition() {
  local partition=$1
  mkfs.xfs -f -n ftype=1 -L DATA ${partition}
  mount -v ${partition} /mnt
  tar -xpvJf /generated/rootfs.tar.xz --strip-components=2 -C /mnt "./var"
  umount -v /mnt
}

function create_extlinux_conf() {
  cat <<EOF >$1
DEFAULT Dianemo
  SAY Dianemo (${VERSION}) by Autonomy
LABEL Dianemo
  KERNEL /boot/vmlinuz
  INITRD /boot/initramfs.xz
  APPEND ip=dhcp consoleblank=0 console=tty0 console=ttyS0,9600 dianemo.autonomy.io/root=${DIANEMO_ROOT} dianemo.autonomy.io/userdata=${DIANEMO_USERDATA} dianemo.autonomy.io/platform=${DIANEMO_PLATFORM}
EOF
}

function cleanup {
  umount 2>/dev/null || true
  partx -d ${DEVICE} 2>/dev/null || true
  losetup -d ${DEVICE} 2>/dev/null || true
}

# Defaults

DIANEMO_ROOT="sda"
DIANEMO_USERDATA=""
DIANEMO_PLATFORM="bare-metal"
RAW_IMAGE="/out/image.raw"
VMDK_IMAGE="/out/image.vmdk"
ISO_IMAGE="/out/image.iso"
FULL=false
RAW=false

case "$1" in
  image)
    shift
    while getopts "b:flt:p:u:" opt; do
      case ${opt} in
        b )
          DEVICE=${OPTARG}
          echo "Using block device ${DEVICE} as installation media"
          ;;
        f )
          echo "Creating full image"
          FULL=true
          ;;
        l )
          trap cleanup ERR
          dd if=/dev/zero of=${RAW_IMAGE} bs=1M count=$(($(size) + 150))
          DEVICE=$(losetup -f)
          RAW=true
          echo "Using loop device ${RAW_IMAGE} as installation media"
          ;;
        p )
          DIANEMO_PLATFORM=${OPTARG}
          echo "Using kernel parameter dianemo.autonomy.io/platform=${DIANEMO_PLATFORM}"
          ;;
        t )
          DIANEMO_ROOT=${OPTARG}
          echo "Using kernel parameter dianemo.autonomy.io/root=${DIANEMO_ROOT}"
          ;;
        u )
          DIANEMO_USERDATA=${OPTARG}
          echo "Using kernel parameter dianemo.autonomy.io/userdata=${DIANEMO_USERDATA}"
          ;;
        \? )
          echo "Invalid Option: -${OPTARG}" 1>&2
          exit 1
          ;;
        : )
          echo "Invalid Option: -${OPTARG} requires an argument" 1>&2
          exit 1
          ;;
      esac
    done
    shift $((OPTIND -1))

  if [ -z "${DIANEMO_USERDATA}" ]; then
      echo "The userdata flag '-u' must be specified"
      exit 1
    fi

    create_image
    ;;
  vmdk)
    create_vmdk
    ;;
  iso)
    create_iso
    ;;
  ami)
    shift
    create_ami "${@}"
    ;;
  *)
      trap - ERR
      exec "$@"
esac
