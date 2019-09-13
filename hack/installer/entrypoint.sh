#!/bin/bash

set -Eeuo pipefail

function create_isolinuxcfg() {

  cat <<EOF >$1
DEFAULT ISO
  SAY Talos
LABEL ISO
  KERNEL /vmlinuz
  INITRD /initramfs.xz
  APPEND page_poison=1 slab_nomerge slub_debug=P pti=on random.trust_cpu=on consoleblank=0 console=tty0 talos.platform=iso
EOF
}

function setup_raw_disk(){
  printf "Creating RAW disk, this may take a moment..."
  if [[ -f ${TALOS_RAW} ]]; then
    rm ${TALOS_RAW}
  fi
  dd if=/dev/zero of="${TALOS_RAW}" bs=1M count=0 seek=${TALOS_RAW_SIZE}
  # NB: Since we use BLKRRPART to tell the kernel to re-read the partition
  # table, it is required to create a partitioned loop device. The BLKRRPART
  # command is meaningful only for partitionable devices.
  DISK=$(losetup --find --partscan --nooverlap --show ${TALOS_RAW})
  printf "done\n"
}

function install_talos() {
  osctl install --bootloader="${WITH_BOOTLOADER}" --disk="${DISK}" --platform="${TALOS_PLATFORM}" --config="${TALOS_CONFIG}" ${EXTRA_ARGS}
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

function create_ova() {
  qemu-img convert -f raw -O vmdk -o compat6,subformat=streamOptimized,adapter_type=lsilogic ${TALOS_RAW} ${TALOS_VMDK}
  # ovf creation
  # reference format: https://www.dmtf.org/standards/ovf
  img_size=$(stat -c %s ${TALOS_VMDK})
  sed -e 's/{{FILESIZE}}/'${img_size}'/' \
      -e 's/{{RAWSIZE}}/'${TALOS_RAW_SIZE}'/' /template.ovf > ${TALOS_OVF}
  sha256sum ${TALOS_VMDK} ${TALOS_OVF} | awk '{ split($NF, filename, "/"); print "SHA256("filename[length(filename)]")= "$1 }' > ${TALOS_MF}
  tar -cf ${TALOS_OVA} -C /out $(basename ${TALOS_OVF}) $(basename ${TALOS_MF}) $(basename ${TALOS_VMDK})
  rm ${TALOS_VMDK} ${TALOS_OVF} ${TALOS_MF}
}

function cleanup {
  umount 2>/dev/null || true
  losetup -d ${DISK} 2>/dev/null || true
}

function usage() {
  printf "entrypoint.sh -p <platform> -u <userdata> [b|d|l|n]"
}

TALOS_RAW_SIZE=544
TALOS_RAW="/out/talos.raw"
TALOS_ISO="/out/talos.iso"
TALOS_VMDK="/out/talos.vmdk"
TALOS_OVF="/out/talos.ovf"
TALOS_MF="/out/talos.mf"
TALOS_OVA="/out/talos.ova"
TALOS_PLATFORM="metal"
TALOS_CONFIG="none"
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
          DISK=${OPTARG}
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
          setup_raw_disk
          ;;
        u )
          TALOS_CONFIG=${OPTARG}
          echo "Using kernel parameter talos.config=${TALOS_CONFIG}"
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

    if [ ! "${TALOS_PLATFORM}" ] || [ ! "${TALOS_CONFIG}" ]; then
      usage
      exit 1
    fi

    trap cleanup EXIT
    echo "Using disk ${DISK} as installation media"
    install_talos
    ;;
  iso)
    create_iso
    ;;
  ova)
    create_ova
    ;;
  ami)
    shift
    create_ami "${@}"
    ;;
  *)
      exec "$@"
esac
