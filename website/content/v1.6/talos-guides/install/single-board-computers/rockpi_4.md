---
title: "Radxa ROCK PI 4"
description: "Installing Talos on Radxa ROCK PI 4a/4b SBC using raw disk image."
aliases:
  - ../../../single-board-computers/rockpi_4c
---

## Prerequisites

You will need

- `talosctl`
- an SD card or an eMMC or USB drive or an nVME drive

Download the latest `talosctl`.

```bash
curl -Lo /usr/local/bin/talosctl https://github.com/siderolabs/talos/releases/download/{{< release >}}/talosctl-$(uname -s | tr "[:upper:]" "[:lower:]")-amd64
chmod +x /usr/local/bin/talosctl
```

## Download the Image

Download the image and decompress it:

```bash
curl -LO https://github.com/siderolabs/talos/releases/download/{{< release >}}/metal-rockpi_4-arm64.raw.xz
xz -d metal-rockpi_4-arm64.raw.xz
```

## Writing the Image

The path to your SD card/eMMC/USB/nVME can be found using `fdisk` on Linux or `diskutil` on macOS.
In this example, we will assume `/dev/mmcblk0`.

Now `dd` the image to your SD card:

```bash
sudo dd if=metal-rockpi_4-arm64.img of=/dev/mmcblk0 conv=fsync bs=4M
```

The user has two options to proceed:

- booting from a SD card or eMMC
- booting from a USB or nVME (requires the RockPi board to have the SPI flash)

### Booting from SD card or eMMC

Insert the SD card into the board, turn it on and proceed to [bootstrapping the node](#bootstrapping-the-node).

### Booting from USB or nVME

This requires the user to flash the RockPi SPI flash with u-boot.

This requires the user has access to [crane CLI](https://github.com/google/go-containerregistry/releases), a spare SD card and optionally access to the [RockPi serial console](https://wiki.radxa.com/Rockpi4/dev/serial-console).

- Flash the Rock PI 4c variant of [Debian](https://wiki.radxa.com/Rockpi4/downloads) to the SD card.
- Boot into the debian image
- Check that /dev/mtdblock0 exists otherwise the command will silently fail; e.g. `lsblk`.
- Download u-boot image from talos u-boot:

```bash
mkdir _out
crane --platform=linux/arm64 export ghcr.io/siderolabs/u-boot:v1.3.0-alpha.0-25-g0ac7773 - | tar xf - --strip-components=1 -C _out rockpi_4/rkspi_loader.img
sudo dd if=rkspi_loader.img of=/dev/mtdblock0 bs=4K
```

- Optionally, you can also write Talos image to the SSD drive right from your Rock PI board:

```bash
curl -LO https://github.com/siderolabs/talos/releases/download/{{< release >}}/metal-rockpi_4-arm64.raw.xz
xz -d metal-rockpi_4-arm64.raw.xz
sudo dd if=metal-rockpi_4-arm64.raw.xz of=/dev/nvme0n1
```

- remove SD card and reboot.

After these steps, Talos will boot from the nVME/USB and enter maintenance mode.
Proceed to [bootstrapping the node](#bootstrapping-the-node).

## Bootstrapping the Node

Wait for the console to show you the instructions for bootstrapping the node.
Following the instructions in the console output to connect to the interactive installer:

```bash
talosctl apply-config --insecure --mode=interactive --nodes <node IP or DNS name>
```

Once the interactive installation is applied, the cluster will form and you can then use `kubectl`.

## Retrieve the `kubeconfig`

Retrieve the admin `kubeconfig` by running:

```bash
talosctl kubeconfig
```
