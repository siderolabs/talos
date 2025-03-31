---
title: "Building Custom Talos Images"
description: "How to build a custom Talos image from source."
---

There might be several reasons to build Talos images from source:

* verifying the [image integrity]({{< relref "verifying-images" >}})
* building an image with custom configuration

## Checkout Talos Source

```bash
git clone https://github.com/siderolabs/talos.git
```

If building for a specific release, checkout the corresponding tag:

```bash
git checkout {{< release >}}
```

## Set up the Build Environment

See [Developing Talos]({{< relref "developing-talos" >}}) for details on setting up the buildkit builder.

## Architectures

By default, Talos builds for `linux/amd64`, but you can customize that by passing `PLATFORM` variable to `make`:

```bash
make <target> PLATFORM=linux/arm64 # build for arm64 only
make <target> PLATFORM=linux/arm64,linux/amd64 # build for arm64 and amd64, container images will be multi-arch
```

## Custom `PKGS`

When [customizing Linux kernel]({{< relref "customizing-the-kernel" >}}), the source for the [`siderolabs/pkgs`](https://github.com/siderolabs/pkgs) repository can
be overridden with:

* if you built and pushed only a custom `kernel` package, the reference can be overridden with `PKG_KERNEL` variable: `make <target> PKG_KERNEL=<registry>/<username>/kernel:<tag>`
* if any other single package was customized, the reference can be overridden with `PKG_<pkg>` (e.g. `PKG_IPTABLES`) variable: `make <target> PKG_<pkg>=<registry>/<username>/<pkg>:<tag>`
* if the full `pkgs` repository was built and pushed, the references can be overridden with `PKGS_PREFIX` and `PKGS` variables: `make <target> PKGS_PREFIX=<registry>/<username> PKGS=<tag>`

## Customizations

Some of the build parameters can be customized by passing environment variables to `make`, e.g. `GOAMD64=v1` can be used to build
Talos images compatible with old AMD64 CPUs:

```bash
make <target> GOAMD64=v1
```

## Building Kernel and Initramfs

The most basic boot assets can be built with:

```bash
make kernel initramfs
```

Build result will be stored as `_out/vmlinuz-<arch>` and `_out/initramfs-<arch>.xz`.

## Building Container Images

Talos container images should be pushed to the registry as the result of the build process.

The default settings are:

* `IMAGE_REGISTRY` is set to `ghcr.io`
* `USERNAME` is set to the `siderolabs` (or value of environment variable `USERNAME` if it is set)

The image can be pushed to any registry you have access to, but the access credentials should be stored in `~/.docker/config.json` file (e.g. with `docker login`).

Building and pushing the image can be done with:

```bash
make installer-base PUSH=true IMAGE_REGISTRY=docker.io USERNAME=<username> # ghcr.io/siderolabs/installer-base
make imager PUSH=true IMAGE_REGISTRY=docker.io USERNAME=<username> # ghcr.io/siderolabs/imager
make installer IMAGE_REGISTRY=docker.io USERNAME=<username> # ghcr.io/siderolabs/installer
```

The [local registry]({{< relref "developing-talos" >}}) running on `127.0.0.1:5005` can be used as well to avoid pushing/pulling over the network:

```bash
make installer REGISTRY=127.0.0.1:5005
```

When building `imager` container, by default Talos will include the boot assets for both `amd64` and `arm64` architectures, if building only for single architecture, specify `INSTALLER_ARCH` variable:

```bash
make imager INSTALLER_ARCH=targetarch PLATFORM=linux/amd64
```

## Building ISO

The [ISO image]({{< relref "../talos-guides/install/boot-assets" >}}) is built with the help of `imager` container image, by default `ghcr.io/siderolabs/imager` will be used with the matching tag:

```bash
make iso
```

The ISO image will be stored as `_out/talos-<arch>.iso`.

If ISO image should be built with the custom `imager` image, it can be specified with `IMAGE_REGISTRY`/`USERNAME` variables:

```bash
make iso IMAGE_REGISTRY=docker.io USERNAME=<username>
```

## Building Disk Images

The disk image is built with the help of `imager` container image, by default `ghcr.io/siderolabs/imager` will be used with the matching tag:

```bash
make image-metal
```

Available disk images are encoded in the `image-%` target, e.g. `make image-aws`.
Same as with ISO image, the custom `imager` image can be specified with `IMAGE_REGISTRY`/`USERNAME` variables.
