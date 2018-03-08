# Dianemo

Dianemo is an operating system with a focus on Kubernetes. Some of the key features of
Dianemo are:

- Immutable: Utilizes a design that adheres to immutable infrastructure
  ideology.
- Secure: Zero host-level access helps reduce the attack surface of a Kubernetes
  cluster.
- Minimal: Only the binaries required by Kubernetes are installed.

## Building

### Tools

```bash
cd tools && conform enforce
```

### Kernel

```bash
cd ../kernel && conform enforce
```

### Initramfs

```bash
cd ../initramfs && conform enforce
```

### Rootfs

```bash
cd ../rootfs && conform enforce
```

### RAW disk

```bash
cd ../generate && conform enforce
```

## Developing `init`

The `init` program is a minimal implementation that aims to manage only the
processes required to run Kubernetes.

## Unpacking the `initramfs`

To unpack the initramfs..

```bash
doker run --rm -it dianemo/rootfs:$TAG
cd /rootfs
mkdir out && cd out
xz -d ../initramfs.xz && (while cpio -idmv; do :; done) <../initramfs
 ```

## Updating the kernel .config

To update the kernel config...

```bash
docker run --rm -it dianemo/kernel:$TAG make olddefconfig
```

## Running

The `generate` project outputs a RAW and VMDK disk image. To run the RAW disk:

```bash
qemu-system-x86_64 -drive file=dianemo.raw,format=raw -cpu host -smp cores=8 -m 4096M -nographic
```

> ^C-a x to exit
