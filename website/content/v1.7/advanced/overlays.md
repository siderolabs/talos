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
