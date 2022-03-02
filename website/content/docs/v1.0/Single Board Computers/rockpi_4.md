---
title: "Radxa ROCK PI 4c"
description: "Installing Talos on Radxa ROCK PI 4c SBC using raw disk image."
---

## Prerequisites

You will need

- `talosctl`
- an SD card

Download the latest `talosctl`.

```bash
curl -Lo /usr/local/bin/talosctl https://github.com/talos-systems/talos/releases/latest/download/talosctl-$(uname -s | tr "[:upper:]" "[:lower:]")-amd64
chmod +x /usr/local/bin/talosctl
```

## Download the Image

Download the image and decompress it:

```bash
curl -LO https://github.com/talos-systems/talos/releases/latest/download/metal-rockpi_4-arm64.img.xz
xz -d metal-rockpi_4-arm64.img.xz
```

## Writing the Image

The path to your SD card can be found using `fdisk` on Linux or `diskutil` on macOS.
In this example, we will assume `/dev/mmcblk0`.

Now `dd` the image to your SD card:

```bash
sudo dd if=metal-rockpi_4-arm64.img of=/dev/mmcblk0 conv=fsync bs=4M
```

## Bootstrapping the Node

Insert the SD card to your board, turn it on and wait for the console to show you the instructions for bootstrapping the node.
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

## Boot Talos from an eMMC or SSD Drive

> Note: this is only tested on Rock PI 4c

It is possible to run Talos without any SD cards right from either an eMMC or SSD disk.

The pre-installed SPI loader won't be able to chain Talos u-boot on the device because it's too outdated.

Instead, it is necessary to update u-boot to a more recent version for this process to work.
The Armbian u-boot build for Rock PI 4c has been proved to work: [https://users.armbian.com/piter75/](https://users.armbian.com/piter75/).

### Steps

- Flash the Rock PI 4c variant of [Debian](https://wiki.radxa.com/Rockpi4/downloads) to the SD card.
- Check that /dev/mtdblock0 exists otherwise the command will silently fail; e.g. `lsblk`.
- Download Armbian u-boot and update SPI flash:

```bash
curl -LO https://users.armbian.com/piter75/rkspi_loader-v20.11.2-trunk-v2.img
sudo dd if=rkspi_loader-v20.11.2-trunk-v2.img of=/dev/mtdblock0 bs=4K
```

- Optionally, you can also write Talos image to the SSD drive right from your Rock PI board:

```bash
curl -LO https://github.com/talos-systems/talos/releases/latest/download/metal-rockpi_4-arm64.img.xz
xz -d metal-rockpi_4-arm64.img.xz
sudo dd if=metal-rockpi_4-arm64.img.xz of=/dev/nvme0n1
```

- remove SD card and reboot.

After these steps, Talos will boot from the SSD and enter maintenance mode.
The rest of the flow is the same as running Talos from the SD card.
