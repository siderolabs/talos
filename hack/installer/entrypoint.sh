#!/bin/bash

set -Eeuo pipefail

function create_isolinuxcfg() {

  cat <<EOF >$1
DEFAULT ISO
  SAY Talos
LABEL ISO
  KERNEL /vmlinuz
  INITRD /initramfs.xz
  APPEND page_poison=1 slab_nomerge pti=on random.trust_cpu=on consoleblank=0 console=tty0 talos.platform=iso
EOF
}

function setup_raw_device(){
  printf "Creating RAW device, this may take a moment..."
  if [[ -f ${TALOS_RAW} ]]; then
    rm ${TALOS_RAW}
  fi
  dd if=/dev/zero of="${TALOS_RAW}" bs=1M count=0 seek=544
  # NB: Since we use BLKRRPART to tell the kernel to re-read the partition
  # table, it is required to create a partitioned loop device. The BLKRRPART
  # command is meaningful only for partitionable devices.
  DEVICE=$(losetup --find --partscan --show ${TALOS_RAW})
  printf "done\n"
}

function install_talos() {
  osctl install --bootloader="${WITH_BOOTLOADER}" --device="${DEVICE}" --platform="${TALOS_PLATFORM}" --userdata="${TALOS_USERDATA}" ${EXTRA_ARGS}
}

function create_iso() {
  mkdir -p /mnt/isolinux
  cp -v /usr/lib/syslinux/isolinux.bin /mnt/isolinux/isolinux.bin
  cp -v /usr/lib/syslinux/ldlinux.c32 /mnt/isolinux/ldlinux.c32

  create_isolinuxcfg /mnt/isolinux/isolinux.cfg
  cp -v /usr/install/vmlinuz /mnt/vmlinuz
  cp -v /usr/install/initramfs.xz /mnt/initramfs.xz

  mkdir -p /mnt/usr/install
  cp -v /usr/install/vmlinuz /mnt/usr/install/vmlinuz
  cp -v /usr/install/initramfs.xz /mnt/usr/install/initramfs.xz

  mkisofs -V TALOS -o ${TALOS_ISO} -r -b isolinux/isolinux.bin -c isolinux/boot.cat -no-emul-boot -boot-load-size 4 -boot-info-table /mnt
  isohybrid ${TALOS_ISO}
}

function create_vmdk() {
  qemu-img convert -f raw -O vmdk ${TALOS_RAW} ${TALOS_VMDK}
}

function create_ami() {
  packer build -var "version=${VERSION}" "${@}" /packer.json
}

function cleanup {
  umount 2>/dev/null || true
  losetup -d ${DEVICE} 2>/dev/null || true
}

function usage() {
  printf "entrypoint.sh -p <platform> -u <userdata> [b|d|l|n]"
}

TALOS_RAW="/out/talos.raw"
TALOS_ISO="/out/talos.iso"
TALOS_VMDK="/out/talos.vmdk"
TALOS_PLATFORM="metal"
TALOS_USERDATA="none"
WITH_BOOTLOADER="true"
EXTRA_ARGS=""

case "$1" in
  install)
   shift
    while getopts "bd:n:p:ru:e:" opt; do
      case ${opt} in
        b )
          echo "Creating disk without bootloader installed"
          WITH_BOOTLOADER="false"
          ;;
        d )
          DEVICE=${OPTARG}
          ;;
        e )
          EXTRA_ARGS="${EXTRA_ARGS} --extra-kernel-arg=${OPTARG}"
          ;;
        n )
          TALOS_RAW="/out/${OPTARG}.raw"
          ;;
        p )
          TALOS_PLATFORM=${OPTARG}
          echo "Using kernel parameter talos.platform=${TALOS_PLATFORM}"
          ;;
        r )
          trap cleanup EXIT
          setup_raw_device
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
    shift $((OPTIND-1))

    if [ ! "${TALOS_PLATFORM}" ] || [ ! "${TALOS_USERDATA}" ]; then
      usage
      exit 1
    fi

    trap cleanup EXIT
    echo "Using device ${DEVICE} as installation media"
    install_talos
    ;;
  iso)
    create_iso
    ;;
  vmdk)
    create_vmdk
    ;;
  ami)
    shift
    create_ami "${@}"
    ;;
  *)
      exec "$@"
esac
