---
title: "ODroid HC4"
description: "Installing Talos on an ODroid HC4 SBC using raw disk image."
aliases:
  - ../../../single-board-computers/odroid_hc4
---
## Prerequisites

You will need:

- `talosctl`
- A microSD card to prepare the unit with
- An additional microSD card, USB stick, or SATA drive to boot Talos from

Download the latest `talosctl`:

```shell
curl -Lo /usr/local/bin/talosctl https://github.com/siderolabs/talos/releases/download/v1.4.0/talosctl-$(uname -s | tr "[:upper:]" "[:lower:]")-amd64

chmod +x /usr/local/bin/talosctl
```

## Download and Write the Image

Download the image and decompress it:

```shell
curl -LO https://github.com/siderolabs/talos/releases/download/v1.4.0/metal-odroid_hc4-arm64.img.xz | xz -d -

xz -d metal-odroid_hc4-arm64.img.xz
```

Write the image to the chosen boot media via [BalenaEtcher](https://www.balena.io/etcher) or `dd`.

## Prepare the Unit

### Erase factory bootloader

- Boot device to petitboot (no USB, SDCard, etc.), the default bootloader
- In petitboot menu, select `Exit to shell`
- Run the following:

```shell
flash_eraseall /dev/mtd0
flash_eraseall /dev/mtd1
flash_eraseall /dev/mtd2
flash_eraseall /dev/mtd3
```

- Power cycle the device

### Install u-boot to SPI

- Flash [Armbian](https://www.armbian.com/odroid-hc4/) to a micro SD card with `dd` or [BalenaEtcher](https://www.balena.io/etcher).
  **A bootable USB stick or SATA drive will not work for this step**
- Insert the Armbian micro SD card into the unit and power it on.
- Once Armbian is booted, install `crane` via the following commands.
  Make sure the unit is connected to the Internet:

```shell
curl -L https://api.github.com/repos/google/go-containerregistry/releases/latest |
  jq -r '.assets[] | select(.name | contains("Linux_arm64")) | .browser_download_url' |
  xargs curl -sL |
  tar zxvf - && \
  install crane /usr/bin
```

- Extract the `u-boot` image from the Talos installer:

```shell
mkdir _out
crane --platform=linux/arm64 export ghcr.io/siderolabs/u-boot:v1.4.0 - | tar xf - --strip-components=1 -C _out odroid_hc4/u-boot.bin
```

- Write `u-boot.bin` to the HC4's SPI/bootloader:

```shell
dd if=_out/u-boot.bin of=/dev/mtdblock0 conv=fsync status=progress
```

- Power off the unit

## Recover Factory Bootloader

**Note:** Only perform these steps if you want to use Hardkernel-published distributions.
Performing these actions will render the unit unable to boot Talos unless the steps in this guide are repeated.

Taken from [the ODroid forums](https://forum.odroid.com/viewtopic.php?t=40906):

- Download the latest `spiupdate` and `spiboot` archives from [here](http://ppa.linuxfactory.or.kr/images/petitboot/odroidhc4/).
- Flash the `spiupdate` image to a microSD card via `dd` or `dd` or [BalenaEtcher](https://www.balena.io/etcher).
- Decompress the `spiboot` archive and copy the file it contains to the flashed SD card.
  Ensure the file is named `spiboot.img`.
- Insert the microSD card into the unit and power it on
