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
      parted -s -a optimal ${RAW_IMAGE} mkpart primary fat32 0 $((${INITRAMFS_SIZE} + 50))M
      parted ${RAW_IMAGE} name 1 ESP
      parted -s -a optimal ${RAW_IMAGE} mkpart primary xfs $((${INITRAMFS_SIZE} + 50))M $((${ROOTFS_SIZE} + ${INITRAMFS_SIZE} + 100))M
      parted ${RAW_IMAGE} name 2 ROOT
      parted -s -a optimal ${RAW_IMAGE} mkpart primary xfs $((${ROOTFS_SIZE} + ${INITRAMFS_SIZE} + 100))M 100%
      parted ${RAW_IMAGE} name 3 DATA
      losetup ${DEVICE} ${RAW_IMAGE}
      partx -av ${DEVICE}
      extract_boot_partition ${DEVICE}p1
      extract_root_partition ${DEVICE}p2
      extract_data_partition ${DEVICE}p3
    else
      parted -s -a optimal ${DEVICE} mkpart primary fat32 0 $((${INITRAMFS_SIZE} + 50))M
      parted ${DEVICE} name 1 ESP
      parted -s -a optimal ${DEVICE} mkpart primary xfs $((${INITRAMFS_SIZE} + 50))M $((${ROOTFS_SIZE} + ${INITRAMFS_SIZE} + 100))M
      parted ${DEVICE} name 2 ROOT
      parted -s -a optimal ${DEVICE} mkpart primary xfs $((${ROOTFS_SIZE} + ${INITRAMFS_SIZE} + 100))M 100%
      parted ${DEVICE} name 3 DATA
      extract_boot_partition ${DEVICE}1
      extract_root_partition ${DEVICE}2
      extract_data_partition ${DEVICE}3
    fi
  else
    if [ "$RAW" = true ] ; then
      parted -s -a optimal ${RAW_IMAGE} mkpart primary xfs 0 $((${ROOTFS_SIZE} + 50))M
      parted ${RAW_IMAGE} name 1 ROOT
      parted -s -a optimal ${RAW_IMAGE} mkpart primary xfs $((${ROOTFS_SIZE} + 50))M 100%
      parted ${RAW_IMAGE} name 2 DATA
      losetup ${DEVICE} ${RAW_IMAGE}
      partx -av ${DEVICE}
      extract_root_partition ${DEVICE}p1
      extract_data_partition ${DEVICE}p2
    else
      parted -s -a optimal ${DEVICE} mkpart primary xfs 0 $((${ROOTFS_SIZE} + 50))M
      parted ${DEVICE} name 1 ROOT
      parted -s -a optimal ${DEVICE} mkpart primary xfs $((${ROOTFS_SIZE} + 50))M 100%
      parted ${DEVICE} name 2 DATA
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
  cp -v /usr/local/src/syslinux/bios/core/isolinux.bin /mnt/boot/isolinux/isolinux.bin
  cp -v /usr/local/src/syslinux/bios/com32/elflink/ldlinux/ldlinux.c32 /mnt/boot/isolinux/ldlinux.c32
  create_extlinux_conf /mnt/boot/isolinux/isolinux.conf
  tar -xpvzf /generated/rootfs.tar.gz -C /mnt
  mkisofs -o ${ISO_IMAGE} -b boot/isolinux/isolinux.bin -c boot/isolinux/boot.cat -no-emul-boot -boot-load-size 4 -boot-info-table .
}

function create_ami() {
  packer build -var "version=${VERSION}" "${@}" /packer.json
}

function size_xz() {
  xz --robot --list $1 | awk 'END { printf("%.0f", $5*0.000001)}'
}

function size_gz() {
  gzip --list $1 | awk 'END { printf("%.0f", $2*0.000001)}'
}

function extract_boot_partition() {
  local partition=$1
  mkfs.vfat ${partition}
  mount -v ${partition} /mnt
  mkdir -pv /mnt/boot/extlinux
  extlinux --install /mnt/boot/extlinux
  create_extlinux_conf /mnt/boot/extlinux/extlinux.conf
  cp -v /generated/boot/vmlinuz /mnt/boot
  cp -v /generated/boot/initramfs.xz /mnt/boot
  umount -v /mnt
}

function extract_root_partition() {
  local partition=$1
  mkfs.xfs -f -n ftype=1 -L ROOT ${partition}
  mount -v ${partition} /mnt
  tar -xpvzf /generated/rootfs.tar.gz --exclude="./var" -C /mnt
  umount -v /mnt
}

function extract_data_partition() {
  local partition=$1
  mkfs.xfs -f -n ftype=1 -L DATA ${partition}
  mount -v ${partition} /mnt
  tar -xpvzf /generated/rootfs.tar.gz --strip-components=2 -C /mnt "./var"
  umount -v /mnt
}

function create_extlinux_conf() {
  # AWS recommends setting the nvme_core.io_timeout to the highest value possible.
  # See https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/nvme-ebs-volumes.html.
  cat <<EOF >$1
DEFAULT Talos
  SAY Talos (${VERSION})
LABEL Talos
  KERNEL /boot/vmlinuz
  INITRD /boot/initramfs.xz
  APPEND ${KERNEL_SELF_PROTECTION_PROJECT_KERNEL_PARAMS} ${EXTRA_KERNEL_PARAMS} nvme_core.io_timeout=4294967295 consoleblank=0 console=tty0 console=ttyS0,9600 talos.userdata=${TALOS_USERDATA} talos.platform=${TALOS_PLATFORM}
EOF
}

function cleanup {
  umount 2>/dev/null || true
  partx -d ${DEVICE} 2>/dev/null || true
  losetup -d ${DEVICE} 2>/dev/null || true
}

# Defaults

TALOS_USERDATA=""
TALOS_PLATFORM=""
RAW_IMAGE="/out/disk.raw"
VMDK_IMAGE="/out/disk.vmdk"
ISO_IMAGE="/out/disk.iso"
FULL=false
RAW=false
TOTAL_SIZE=$(size_gz /generated/rootfs.tar.gz)
ROOTFS_SIZE=$(tar -tvf /generated/rootfs.tar.gz --exclude=./var | awk '{s+=$3} END { printf "%.0f", s*0.000001}')
INITRAMFS_SIZE=$(size_xz /generated/boot/initramfs.xz)
# TODO(andrewrynhard): Add slub_debug=P. See https://github.com/talos-systems/talos/pull/157.
KERNEL_SELF_PROTECTION_PROJECT_KERNEL_PARAMS="page_poison=1 slab_nomerge pti=on"
EXTRA_KERNEL_PARAMS=""

case "$1" in
  disk)
    shift
    while getopts "b:fln:p:u:e:" opt; do
      case ${opt} in
        b )
          DEVICE=${OPTARG}
          echo "Using block device ${DEVICE} as installation media"
          ;;
        e )
          EXTRA_KERNEL_PARAMS=${OPTARG}
          echo "Using extra kernel params ${EXTRA_KERNEL_PARAMS}"
          ;;
        f )
          echo "Creating full disk"
          FULL=true
          ;;
        l )
          trap cleanup ERR
          dd if=/dev/zero of=${RAW_IMAGE} bs=1M count=$(($TOTAL_SIZE+$INITRAMFS_SIZE+150))
          DEVICE=$(losetup -f)
          RAW=true
          echo "Using loop device ${RAW_IMAGE} as installation media"
          ;;
        n )
          RAW_IMAGE="/out/${OPTARG}.raw"
          ;;
        p )
          TALOS_PLATFORM=${OPTARG}
          echo "Using kernel parameter talos.platform=${TALOS_PLATFORM}"
          ;;
        u )
          TALOS_USERDATA=${OPTARG}
          echo "Using kernel parameter talos.userdata=${TALOS_USERDATA}"
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

    if [ "$FULL" = true ] ; then
      if [ -z "${TALOS_PLATFORM}" ]; then
        echo "The platform flag '-p' must be specified"
        exit 1
      fi

      if [ -z "${TALOS_USERDATA}" ]; then
        echo "The userdata flag '-u' must be specified"
        exit 1
      fi
    fi
    echo -e "Creating disk\n\t/: ${ROOTFS_SIZE}Mb\n\t/boot: ${INITRAMFS_SIZE}Mb"
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
