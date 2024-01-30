---
title: "Customizing the Kernel"
description: "Guide on how to customize the kernel used by Talos Linux."
aliases:
  - ../guides/customizing-the-kernel
---

Talos Linux configures the kernel to allow loading only cryptographically signed modules.
The signing key is generated during the build process, it is unique to each build, and it is not available to the user.
The public key is embedded in the kernel, and it is used to verify the signature of the modules.
So if you want to use a custom kernel module, you will need to build your own kernel, and all required kernel modules in order to get the signature in sync with the kernel.

## Overview

In order to build a custom kernel (or a custom kernel module), the following steps are required:

- build a new Linux kernel and modules, push the artifacts to a registry
- build a new Talos base artifacts: kernel and initramfs image
- produce a new Talos boot artifact (ISO, installer image, disk image, etc.)

We will go through each step in detail.

## Building a Custom Kernel

First, you might need to prepare the build environment, follow the [Building Custom Images]({{< relref "building-images" >}}) guide.

Checkout the [`siderolabs/pkgs`](https://github.com/siderolabs/pkgs) repository:

```shell
git clone https://github.com/siderolabs/pkgs.git
cd pkgs
git checkout {{< release_branch >}}
```

The kernel configuration is located in the files `kernel/build/config-ARCH` files.
It can be modified using the text editor, or by using the Linux kernel `menuconfig` tool:

```shell
make kernel-menuconfig
```

The kernel configuration can be cleaned up by running:

```shell
make kernel-olddefconfig
```

Both commands will output the new configuration to the `kernel/build/config-ARCH` files.

Once ready, build the kernel any out-of-tree modules (if required, e.g. `zfs`) and push the artifacts to a registry:

```shell
make kernel REGISTRY=127.0.0.1:5005 PUSH=true
```

By default, this command will compile and push the kernel both for `amd64` and `arm64` architectures, but you can specify a single architecture by overriding
a variable `PLATFORM`:

```shell
make kernel REGISTRY=127.0.0.1:5005 PUSH=true PLATFORM=linux/amd64
```

This will create a container image `127.0.0.1:5005/siderolabs/kernel:$TAG` with the kernel and modules.

## Building Talos Base Artifacts

Follow the [Building Custom Images]({{< relref "building-images" >}}) guide to set up the Talos source code checkout.

If some new kernel modules were introduced, adjust the list of the default modules compiled into the Talos `initramfs` by
editing the file `hack/modules-ARCH.txt`.

Try building base Talos artifacts:

```shell
make kernel initramfs PKG_KERNEL=127.0.0.1:5005/siderolabs/kernel:$TAG PLATFORM=linux/amd64
```

This should create a new image of the kernel and initramfs in `_out/vmlinuz-amd64` and `_out/initramfs-amd64.xz` respectively.

> Note: if building for `arm64`, replace `amd64` with `arm64` in the commands above.

As a final step, produce the new `imager` container image which can generate Talos boot assets:

```shell
make imager PKG_KERNEL=127.0.0.1:5005/siderolabs/kernel:$TAG PLATFORM=linux/amd64 INSTALLER_ARCH=targetarch
```

> Note: if you built the kernel for both `amd64` and `arm64`, a multi-arch `imager` container can be built as well by specifying `INSTALLER_ARCH=all` and `PLATFORM=linux/amd64,linux/arm64`.

## Building Talos Boot Assets

Follow the [Boot Assets]({{< relref "../talos-guides/install/boot-assets" >}}) guide to build Talos boot assets you might need to boot Talos: ISO, `installer` image, etc.
Replace the reference to the `imager` in guide with the reference to the `imager` container built above.

> Note: if you update the `imager` container, don't forget to `docker pull` it, as `docker` caches pulled images and won't pull the updated image automatically.
