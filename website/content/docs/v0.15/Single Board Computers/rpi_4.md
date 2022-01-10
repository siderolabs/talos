---
title: "Raspberry Pi 4 Model B"
description: "Installing Talos on Rpi4 SBC using raw disk image."
---

## Video Walkthrough

To see a live demo of this writeup, see the video below:
<iframe width="560" height="315" src="https://www.youtube.com/embed/aHu1lFir7UU" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

## Prerequisites

You will need

- `talosctl`
- an SD card

Download the latest alpha `talosctl`.

```bash
curl -Lo /usr/local/bin/talosctl https://github.com/talos-systems/talos/releases/latest/download/talosctl-$(uname -s | tr "[:upper:]" "[:lower:]")-amd64
chmod +x /usr/local/bin/talosctl
```

## Updating the EEPROM

At least version `v2020.09.03-138a1` of the bootloader (`rpi-eeprom`) is required.
To update the bootloader we will need an SD card.
Insert the SD card into your computer and use [Raspberry Pi Imager](https://www.raspberrypi.org/software/)
to install the bootloader on it (select Operating System > Misc utility images > Bootloader > SD Card Boot).
Alternatively, you can use the console on Linux or macOS.
The path to your SD card can be found using `fdisk` on Linux or `diskutil` on macOS.
In this example, we will assume `/dev/mmcblk0`.

```bash
curl -Lo rpi-boot-eeprom-recovery.zip https://github.com/raspberrypi/rpi-eeprom/releases/download/v2021.04.29-138a1/rpi-boot-eeprom-recovery-2021-04-29-vl805-000138a1.zip
sudo mkfs.fat -I /dev/mmcblk0
sudo mount /dev/mmcblk0p1 /mnt
sudo bsdtar rpi-boot-eeprom-recovery.zip -C /mnt
```

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
curl -LO https://github.com/talos-systems/talos/releases/latest/download/metal-rpi_4-arm64.img.xz
xz -d metal-rpi_4-arm64.img.xz
```

## Writing the Image

Now `dd` the image to your SD card:

```bash
sudo dd if=metal-rpi_4-arm64.img of=/dev/mmcblk0 conv=fsync bs=4M
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
