---
title: "Overlays"
description: "Overlays"
---

Overlays provide a way to customize Talos Linux boot image.
Overlays hook into the Talos install steps and can be used to provide additional boot assets (in the case of single board computers),
extra kernel arguments or some custom configuration that is not part of the default Talos installation and specific to a particular overlay.

## Overlays v/s Extensions

Overlays are similar to extensions, but they are used to customize the installation process, while extensions are used to customize the root filesystem.

## Official Overlays

The list of official overlays can be found in the [Overlays GitHub repository](https://github.com/siderolabs/overlays/).

## Using Overlays

Overlays can be used to generate a modified metal image or installer image with the overlay applied.

The process of generating boot assets with overlays is described in the [boot assets guide]({{< relref "../talos-guides/install/boot-assets" >}}).

### Example: Booting a Raspberry Pi 4 with an Overlay

Follow the board specific guide for [Raspberry Pi]({{< relref "../talos-guides/install/single-board-computers/rpi_generic" >}}) to download or generate the metal disk image and write to an SD card.

Boot the machine with the boot media and apply the machine configuration with the installer image that has the overlay applied.

```yaml
# Talos machine configuration patch
machine:
  install:
    image: factory.talos.dev/installer/fc1cceeb5711cd263877b6b808fbf4942a8deda65e8804c114a0b5bae252dc50:{{< release >}}
```

> Note: The schematic id shown in the above patch is for a vanilla `rpi_generic` overlay.
> Replace it with the schematic id of the overlay you want to apply.

## Authoring Overlays

An Overlay is a container image with the [specific folder structure](https://github.com/siderolabs/overlays#readme).
Overlays can be built and managed using any tool that produces container images, e.g. `docker build`.

Sidero Labs maintains a [repository of overlays](https://github.com/siderolabs/overlays).

### Developing An Overlay

Let's assume that you would like to contribute an overlay for a specific board, e.g. by contributing to the [`sbc-rockchip` repository](https://github.com/siderolabs/sbc-rockchip).
Clone the repositry and insepct the existing overlays to understand the structure.

Usually an overlay consist of a few key components:

- `firmware`: contains the firmware files required for the board
- `bootloader`: contains the bootloader, e.g. `u-boot` for the board
- `dtb`: contains the device tree blobs for the board
- `installer`: contains the installer that will be used to install this overlay on the node
- `profile`: contains information about the disk image profile, e.g. the disk image size, bootloader used, output format etc.

1. For the new overlay, create any needed folders and `pkg.yaml` files.
2. If your board introduces a new chipset that is not supported yet, make sure to add the firmware build for it.
3. Add the necessary `u-boot` and `dtb` build steps to the `pkg.yaml` files.
4. Proceed to add an installer, which is a small go binary that will be used to install the overlay on the node.
    Here you need to add the go `src/` as well as the `pkg.yaml` file.
5. Lastly, add the profile information in the `profiles` folder.

You are now ready to attempt building the overlay.
It's recommend to push the build to a container registry to test the overlay with the Talos installer.

The default settings are:

- `REGISTRY` is set to `ghcr.io`
- `USERNAME` is set to the `siderolabs` (or value of environment variable `USERNAME` if it is set)

```bash
make sbc-rockchip PUSH=true
```

If using a custom registry, the `REGISTRY` and `USERNAME` variables can be set:

```bash
make sbc-rockchip PUSH=true REGISTRY=<registry> USERNAME=<username>
```

After building the overlay, take note of the pushed image tag, e.g. `664638a`, because you will need it for the next step.
You can now build a flashable image using the command below.

```bash
export TALOS_VERSION=v1.7.6
export USERNAME=octocat
export BOARD=nanopi-r5s
export TAG=664638a

docker run --rm -t -v ./_out:/out -v /dev:/dev --privileged ghcr.io/siderolabs/imager:${TALOS_VERSION} \
    "${BOARD}" --arch arm64 \
    --base-installer-image="ghcr.io/siderolabs/installer-base:${TALOS_VERSION}" \
    --overlay-name="${BOARD}" \
    --overlay-image="ghcr.io/${USERNAME}/sbc-rockchip:${TAG}" \
```

> **--overlay-option**

 `--overlay-option` can be used to pass additional options to the overlay installer if they are implemented by the overlay.
 An example can be seen in the [sbc-raspberrypi](https://github.com/siderolabs/sbc-raspberrypi/) overlay repository.
 It supports passing multiple options by repeating the flag or can be read from a yaml document by passing `--overlay-option=@<path to file>`.

> **Note:** In some cases you need to build a custom imager.
> In this case, refer to [this guide on how to build a custom images]({{< relref "./building-images" >}}) using an imager.

#### Troubleshooting

> **IMPORTANT:** If this does not succeed, have a look at the documentation of the external dependecies you are pulling in and make sure that the `pkg.yaml` files are correctly configured.
> In some cases it may be required to update the dependencies to an appropriate version via the `Pkgfile`.
