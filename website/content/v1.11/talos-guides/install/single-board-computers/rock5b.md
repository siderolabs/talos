---
title: "Radxa ROCK 5B"
description: "Installing Talos on Radxa ROCK 5B SBC using raw disk image."
aliases:
  - ../../../single-board-computers/rock5b
---

## Prerequisites

You will need

- follow [Installation/talosctl]({{< relref "../talosctl">}}) to intall `talosctl`
- an SD card

## Download the Image

Visit the [Image Factory](https://factory.talos.dev/), select `Single Board Computers`, select the version and select `Radxa ROCK 5B` from the options.

Choose `realtek-firmware` and any other desired extension.
Next fill in the kernel command line arguments if needed.

Download the image and decompress it:

```bash
curl -LO https://factory.talos.dev/image/[uuid]/{{< release >}}/metal-arm64.raw.xz
xz -d metal-arm64.raw.xz
```

## Writing the Image

This guide assumes the node should boot from SD card.
Booting from eMMC or NVMe has not been tested yet.

The path to your SD card can be found using `fdisk` on Linux or `diskutil` on macOS.
In this example, we will assume `/dev/mmcblk0`.

Now `dd` the image to your SD card:

```bash
sudo dd if=metal-arm64.raw of=/dev/mmcblk0 conv=fsync oflag=direct status=progress bs=4M
```

## First boot

Insert the SD card into the board, turn it on and proceed by following the
[getting started guide]({{< relref "../../../introduction/getting-started/#configure-talos-linux" >}})
for further steps on how to configure Talos.

## Troubleshooting

### Serial console

If you experience any issues you can check the serial console.
Follow the [official guideline](https://wiki.radxa.com/Rock5/dev/serial-console)
on how to connect a serial adapter.

Hint: The rock5b overlay uses baudrate of `115200` instead of the default `1500000`

### Power supplies and endless restarts

It is a known issue that USB Power Delivery negotiation is performed at a late stage in kernel.
This can lead to endless restarts if the power supply cuts power to early.
Check the list of [known working](https://wiki.radxa.com/Rock5/5b/power_supply) power supplies.

## Tips and tricks

### EPHEMERAL on NVMe

The Radxa ROCK 5B SBC provides a M.2 NVMe SSD slot.

This allows to use a separate disk for the EPHEMERAL partition by following
[Disk Management]({{< relref "../../configuration/disk-management" >}}).
