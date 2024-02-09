---
title: "Raspberry Pi Series"
description: "Installing Talos on Raspberry Pi SBC's using raw disk image."
aliases:
  - ../../../single-board-computers/rpi_generic
---

Talos disk image for the Raspberry Pi generic should in theory work for the boards supported by [u-boot](https://github.com/u-boot/u-boot/blob/master/doc/board/broadcom/raspberrypi.rst#64-bit) `rpi_arm64_defconfig`.
This has only been officialy tested on the Raspberry Pi 4 and community tested on one variant of the Compute Module 4 using Super 6C boards.
If you have tested this on other Raspberry Pi boards, please let us know.

## Video Walkthrough

To see a live demo of this writeup, see the video below:

<iframe width="560" height="315" src="https://www.youtube.com/embed/aHu1lFir7UU" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

## Prerequisites

You will need

- `talosctl`
- an SD card

Download the latest `talosctl`.

```bash
curl -sL 'https://www.talos.dev/install' | bash
```

## Updating the EEPROM

Use [Raspberry Pi Imager](https://www.raspberrypi.com/software/) to write an EEPROM update image to a spare SD card.
Select Misc utility images under the Operating System tab.

Remove the SD card from your local machine and insert it into the Raspberry Pi.
Power the Raspberry Pi on, and wait at least 10 seconds.
If successful, the green LED light will blink rapidly (forever), otherwise an error pattern will be displayed.
If an HDMI display is attached to the port closest to the power/USB-C port,
the screen will display green for success or red if a failure occurs.
Power off the Raspberry Pi and remove the SD card from it.

> Note: Updating the bootloader only needs to be done once.

## Download the Image

Download the image and decompress it:

```bash
curl -LO https://github.com/siderolabs/talos/releases/download/{{< release >}}/metal-rpi_generic-arm64.raw.xz
xz -d metal-rpi_generic-arm64.raw.xz
```

## Writing the Image

Now `dd` the image to your SD card:

```bash
sudo dd if=metal-rpi_generic-arm64.raw of=/dev/mmcblk0 conv=fsync bs=4M
```

## Bootstrapping the Node

Insert the SD card to your board, turn it on and wait for the console to show you the instructions for bootstrapping the node.
Following the instructions in the console output to connect to the interactive installer:

```bash
talosctl apply-config --insecure --mode=interactive --nodes <node IP or DNS name>
```

Once the interactive installation is applied, the cluster will form and you can then use `kubectl`.

> Note: if you have an HDMI display attached and it shows only a rainbow splash,
> please use the other HDMI port, the one closest to the power/USB-C port.

## Retrieve the `kubeconfig`

Retrieve the admin `kubeconfig` by running:

```bash
talosctl kubeconfig
```

## Troubleshooting

The following table can be used to troubleshoot booting issues:

| Long Flashes | Short Flashes |                              Status |
| ------------ | :-----------: | ----------------------------------: |
| 0            |       3       |             Generic failure to boot |
| 0            |       4       |               start\*.elf not found |
| 0            |       7       |              Kernel image not found |
| 0            |       8       |                       SDRAM failure |
| 0            |       9       |                  Insufficient SDRAM |
| 0            |      10       |                       In HALT state |
| 2            |       1       |                   Partition not FAT |
| 2            |       2       |       Failed to read from partition |
| 2            |       3       |          Extended partition not FAT |
| 2            |       4       | File signature/hash mismatch - Pi 4 |
| 4            |       4       |              Unsupported board type |
| 4            |       5       |                Fatal firmware error |
| 4            |       6       |                Power failure type A |
| 4            |       7       |                Power failure type B |
