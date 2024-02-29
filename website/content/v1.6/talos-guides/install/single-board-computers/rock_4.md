---
title: "Radxa ROCK 4 (A/B/C/C+)"
description: "Installing Talos on Radxa ROCK 4 (A/B/C/C+) SBC using raw disk image."
aliases:
  - ../../../single-board-computers/rock_4
---

## Prerequisites

You will need:

- [Talos configuration files]({{< relref "../../introduction/getting-started/#configure-talos-linux" >}})
- an SD card or an eMMC or USB drive or an NVMe drive

## Download the Image

Download the image for your ROCK variant and decompress it:

### ROCK 4 A/B
```bash
curl -LO https://github.com/siderolabs/talos/releases/download/{{< release >}}/metal-rockpi_4-arm64.raw.xz
xz -d metal-rockpi_4-arm64.raw.xz
```

### ROCK 4 C
```bash
curl -LO https://github.com/siderolabs/talos/releases/download/{{< release >}}/metal-rockpi_4c-arm64.raw.xz
xz -d metal-rockpi_4c-arm64.raw.xz
```

### ROCK 4 C+
```bash
curl -LO https://github.com/siderolabs/talos/releases/download/{{< release >}}/metal-rock_4c_plus-arm64.raw.xz
xz -d metal-rock_4c_plus-arm64.raw.xz
```

## Writing the Image

The path to your SD card/eMMC/USB/NVMe can be found using `fdisk` on Linux or `diskutil` on macOS.

Now copy the image to your media with `dd` (replace `metal-rockpi_4-arm64.img` and `/dev/mmcblk0` from the example with your values):

```bash
sudo dd if=metal-rockpi_4-arm64.img of=/dev/mmcblk0 conv=fsync bs=4M
```

The user has two options to proceed:

- booting from a SD card or eMMC
- booting from a USB or NVMe (requires the ROCK board to have an [SPI flash chip soldered](https://wiki.radxa.com/Rockpi4/hardware/spi_flash))

### Booting from SD card or eMMC

Insert the SD card or eMMC module into the board, turn it on and proceed to [apply the node configuration]({{< relref "../../introduction/getting-started/#apply-configuration" >}}).

### Booting from USB or NVMe

This requires the user to flash U-Boot into the ROCK SPI flash.

Use [crane CLI](https://github.com/google/go-containerregistry/releases), a spare SD card or eMMC module, and optionally access to the [ROCK serial console](https://wiki.radxa.com/Rockpi4/dev/serial-console).

- Flash the ROCK 4 variant of [Debian](https://wiki.radxa.com/Rockpi4/downloads) to the SD card or eMMC module.
- Boot Debian.
- Check that /dev/mtdblock0 exists otherwise the command will silently fail; e.g. `lsblk`.
- Download U-Boot image from Sidero Labs packages:

```bash
mkdir _out
crane --platform=linux/arm64 export ghcr.io/siderolabs/u-boot:v1.3.0-alpha.0-25-g0ac7773 - | tar xf - --strip-components=1 -C _out rockpi_4/rkspi_loader.img
sudo dd if=rkspi_loader.img of=/dev/mtdblock0 bs=4K
```

- Optionally, you can write the Talos image to the USB/NVMe drive right from your ROCK board:

```bash
curl -LO https://github.com/siderolabs/talos/releases/download/{{< release >}}/metal-rockpi_4-arm64.raw.xz
xz -d metal-rockpi_4-arm64.raw.xz
sudo dd if=metal-rockpi_4-arm64.raw.xz of=/dev/nvme0n1
```

- Remove SD card or eMMC module and reboot.

After following these steps, Talos will boot from the NVMe/USB and enter maintenance mode.
Proceed to [apply the node configuration]({{< relref "../../introduction/getting-started/#apply-configuration" >}}).
