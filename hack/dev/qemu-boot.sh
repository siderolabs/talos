#!/bin/bash

set -e

case $(uname -s) in
  Linux*)
    OSCTL="${PWD}/build/osctl-linux-amd64"
    ;;

  Darwin*)
    OSCTL="${PWD}/build/osctl-darwin-amd64"
    ;;
  *)
    exit 1
    ;;
esac


KERNEL="build/vmlinuz"
INITRD="build/initramfs.xz"

TMP=$(mktemp -d)

# trap "rm -rf $TMP" EXIT

cp ${KERNEL} ${INITRD} ${TMP}
cd $TMP

echo "Temporary directory: $TMP"

${OSCTL} config generate --install-image="${1}" kvm 10.254.0.10,10.254.0.11,10.254.0.12

configs=( master-1 master-2 master-3 worker )
for i in "${!configs[@]}"; do
  n=${configs[${i}]}
  mkdir ${n}
  mv ${n}.yaml ${n}/config.yaml
  genisoimage -output ${n}-config.iso -volid metal-iso -joliet -rock ${n}
  qemu-img create -f qcow2 ${n}-rootfs.qcow2 8G
  qemu-system-x86_64 \
      -pidfile ${TMP}/${n}.pid \
      -m 2048 \
      -accel ${ACCEL},thread=multi \
      -cpu max \
      -smp 2 \
      -hda ${n}-rootfs.qcow2 \
      -drive file=${n}-config.iso,media=cdrom \
      -netdev user,id=talos,ipv4=on,net=10.254.0.0/24,dhcpstart=10.254.0.10,hostfwd=tcp::$((50000+${i}))-:50000,hostfwd=tcp::$((6443+${i}))-:6443,hostname=${n} \
      -device virtio-net,netdev=talos \
      -display none \
      -serial file:${TMP}/${n}.log \
      -append "page_poison=1 slub_debug=P slab_nomerge pti=on random.trust_cpu=on printk.devkmsg=on earlyprintk=serial,tty0,keep console=tty0 talos.platform=metal talos.config=metal-iso" \
      -kernel vmlinuz \
      -initrd initramfs.xz \
      -daemonize
done

${OSCTL} config generate --install-image="${1}" kvm 10.254.0.10

function setup() {
  # Create the RAW disk.
  docker run --rm -v /dev:/dev -v ${PWD}:/out \
      --privileged \
      autonomy/installer:6c33547-dirty \
      install \
      -n kvm \
      -r \
      -p metal \
      -u metal-iso \
      -e console=tty1 \
      -e console=ttyS0

  # Convert the RAW disk to qcow2 and expand it.

  qemu-img convert -f raw -O qcow2 ${PWD}/kvm.raw ${PWD}/kvm.qcow2
  qemu-img resize ${PWD}/kvm.qcow2 +8G
}

function run() {
  for n in master-1 master-2 master-3 worker; do
    # Create the configuration ISO.

    mkdir ${n}
    mv ${n}.yaml ${n}/config.yaml
    CONFIG_ISO="${n}-config.iso"
    genisoimage -output ${CONFIG_ISO} -volid metal-iso -joliet -rock ${n}

    # Create the qcow2 disk.

    QCOW2_IMAGE="${n}-rootfs.qcow2"
    cp kvm.qcow2 ${QCOW2_IMAGE}

    # Create the VM.

    virt-install \
        -n ${n} \
        --os-type=Linux \
        --os-variant=generic \
        --virt-type=kvm \
        --cpu=host \
        --vcpus=2 \
        --ram=2048 \
        --disk path=${QCOW2_IMAGE},format=qcow2,bus=virtio,cache=none \
        --disk path=${CONFIG_ISO},device=cdrom \
        --network bridge=br0,model=e1000 \
        --graphics none \
        --boot hd \
        --rng /dev/random
  done
}

setup
run

# configs=( master-1 master-2 master-3 worker )
# for i in "${!configs[@]}"; do
#   n=${configs[${i}]}
#   mkdir ${n}
#   mv ${n}.yaml ${n}/config.yaml
#   genisoimage -output ${n}-config.iso -volid metal-iso -joliet -rock ${n}
#   qemu-img create -f qcow2 ${n}-rootfs.qcow2 8G
#   qemu-system-x86_64 \
#       -pidfile ${TMP}/${n}.pid \
#       -m 2048 \
#       -accel kvm,hvf,thread=multi \
#       -cpu max \
#       -smp 2 \
#       -hda ${n}-rootfs.qcow2 \
#       -drive file=${n}-config.iso,media=cdrom \
#       -netdev user,id=talos,ipv4=on,net=10.254.0.0/24,dhcpstart=10.254.0.10,hostfwd=tcp::$((50000+${i}))-:50000,hostfwd=tcp::$((6443+${i}))-:6443,hostname=${n} \
#       -device virtio-net,netdev=talos \
#       -display none \
#       -serial file:${TMP}/${n}.log \
#       -append "page_poison=1 slub_debug=P slab_nomerge pti=on random.trust_cpu=on printk.devkmsg=on earlyprintk=serial,tty0,keep console=tty0 talos.platform=metal talos.config=metal-iso" \
#       -kernel vmlinuz \
#       -initrd initramfs.xz \
#       -daemonize
# done
