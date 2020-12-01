---
title: "Raspberry Pi 4 Model B"
---

## Updating the Bootloader

A recent version of the `rpi-eeprom` is required.
To update the bootloader we will need an SD card.
Insert the SD card into your computer and run the following:

```bash
curl -LO https://github.com/raspberrypi/rpi-eeprom/releases/download/v2020.09.03-138a1/rpi-boot-eeprom-recovery-2020-09-03-vl805-000138a1.zip
sudo mkfs.fat -I /dev/mmcblk0
sudo mount /dev/mmcblk0 /mnt
sudo bsdtar rpi-boot-eeprom-recovery-2020-09-03-vl805-000138a1.zip -C /mnt
```

Insert the SD card into the Raspberry Pi, power it on, and wait at least 10 seconds.
If successful, the green LED light will blink rapidly (forever), otherwise an error pattern will be displayed.
If a HDMI display is attached then screen will display green for success or red if failure a failure occurs.
Power off the Raspberry Pi and remove the SD card.

## Download the Image

An official image is provided in a release.
Download the tarball and extract the image:

```bash
curl -LO https://github.com/talos-systems/talos/releases/download/<version>/metal-rpi_4-arm64.tar.gz
tar -xvf metal-rpi_4-arm64.tar.gz
```

## Writing the Image

Now `dd` the image your SD card (be sure to update `x` in `mmcblkx`):

```bash
sudo dd if=disk.raw of=/dev/mmcblkx
```

## Bootstrapping the Node

Insert the SD card, turn on the board, and wait for the console to show you the instructions for bootstrapping the node.
Following the instructions in the console output, generate the configuration files and apply the `init.yaml`:

```bash
talosctl gen config example https://<node IP or DNS name>:6443
talosctl apply-config --insecure --file init.yaml --nodes <node IP or DNS name>
```

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
