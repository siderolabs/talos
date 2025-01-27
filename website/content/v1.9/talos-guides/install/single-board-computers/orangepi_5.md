---
title: "Orange Pi 5"
description: "Installing Talos on Orange Pi 5 using raw disk image."
aliases:
  - ../../../single-board-computers/orangepi_5
---

## Prerequisites

Before you start:

- follow [Installation/talosctl]({{< relref "../talosctl">}}) to intall `talosctl`

## Boot options

You can boot Talos from:

1. booting from SD card
2. booting from a USB or NVMe (requires a spi image on the SPI flash)

### Booting from SD card

Go to `https://factory.talos.dev` select `Single Board Computers`, select the version and select `Orange Pi 5` from the options.
Choose your desired extensions and fill in the kernel command line arguments if needed.

Download the disk image and decompress it:

```bash
curl -LO https://factory.talos.dev/image/[uuid]/{{< release >}}/metal-arm64.raw.xz
xz -d metal-arm64.raw.xz
```

#### Flash the Image

The image can be flashed using Etcher on Windows, macOS, or Linux or using dd on Linux:

```bash
# Replace /dev/<device> with the destination device
# You can find the device with `lsblk` or `fdisk -l`
sudo dd if=metal-arm64.raw of=/dev/<device> bs=1M status=progress && sync
```

Proceed by following the [getting started guide]({{< relref "../../../introduction/getting-started/#configure-talos-linux" >}}) for further steps on how to configure Talos.

#### Booting from USB or NVMe

#### Requirements

- An SD card to boot the Orange Pi 5 board from in order to flash the SPI flash.

Go to `https://factory.talos.dev` select `Single Board Computers`, select the version and select `Orange Pi 5` from the options.
Choose your desired extensions and fill in the kernel command line arguments if needed.

You should also add the `spi_boot: true` overlay extra option in order to remove u-boot from the final image, as the bootloader will be flashed to the SPI flash.

Download the disk image and decompress it:

```bash
curl -LO https://factory.talos.dev/image/[uuid]/{{< release >}}/metal-arm64.raw.xz
xz -d metal-arm64.raw.xz
```

#### Steps

1. Make sure to install the NVMe or USB drive in the Orange Pi 5 board.

2. Boot the Orange Pi 5 board from the SD card:

    - Flash the Orange Pi 5 variant of [Ubuntu](http://www.orangepi.org/html/hardWare/computerAndMicrocontrollers/service-and-support/Orange-pi-5.html) to an SD card.
    - Insert the SD card into the Orange Pi 5 board.
    - Boot into the Ubuntu image.
    - Download [crane CLI](https://github.com/google/go-containerregistry/releases) on the Ubuntu image.

3. From the Ubuntu image, find the latest `sbc-rockchip` overlay, download and extract the u-boot SPI image:

    - Find the latest release tag of the [sbc-rockchip repo](https://github.com/siderolabs/sbc-rockchip/releases).
    - Download and extract the u-boot SPI image:

      ```bash
      crane --platform=linux/arm64 export ghcr.io/siderolabs/sbc-rockchip:<releasetag> | tar x --strip-components=4 artifacts/arm64/u-boot/orangepi-5/u-boot-rockchip-spi.bin
      ```

4. Flash the SPI flash with the u-boot SPI image:

    ```bash
    devicesize=$(blockdev --getsz /dev/mtdblock0)
    dd if=/dev/zero of=/dev/mtdblock0 bs=1M count=$devicesize status=progress && sync
    dd if=u-boot-rockchip-spi.bin of=/dev/mtdblock0 bs=1M status=progress && sync
    ```

5. Flash the Talos raw image to the NVMe or USB drive:

    ```bash
    sudo dd if=metal-arm64.raw of=/dev/<device> bs=1M status=progress && sync
    ```

6. Shutdown the Orange Pi 5 board and remove the SD card.

On the next boot, Talos will now boot from the NVMe/USB and enter maintenance mode.

Proceed by following the [getting started guide]({{< relref "../../../introduction/getting-started/#configure-talos-linux" >}}) for further steps on how to configure Talos.

## Troubleshooting

### Serial console

If you experience any issues you can check the serial console.
Follow the [official guideline](https://drive.google.com/drive/folders/1ob_qOW2MMa7oncxIW6625NqwXHRxdeAo) (Section 2.18 — "How to use the debugging serial port")
on how to connect a serial adapter.
