---
title: "Jetson Nano"
description: "Installing Talos on Jetson Nano SBC using raw disk image."
---

## Prerequisites

You will need

- `talosctl`
- an SD card/USB drive
- [crane CLI](https://github.com/google/go-containerregistry/releases)

Download the latest `talosctl`.

```bash
curl -Lo /usr/local/bin/talosctl https://github.com/talos-systems/talos/releases/latest/download/talosctl-$(uname -s | tr "[:upper:]" "[:lower:]")-amd64
chmod +x /usr/local/bin/talosctl
```

## Flashing the firmware to on-board SPI flash

> Flashing the firmware only needs to be done once.

We will use the [R32.6.1 release](https://developer.nvidia.com/embedded/l4t/r32_release_v6.1/t210/jetson-210_linux_r32.6.1_aarch64.tbz2) for the Jetson Nano.
Most of the instructions is similar to this [doc](https://nullr0ute.com/2020/11/installing-fedora-on-the-nvidia-jetson-nano/) except that we'd be using a patched version of `u-boot` so that USB boot also works.

Before flashing we need the following:

- A USB-A to micro USB cable
- A jumper wire to enable recovery mode
- A HDMI monitor to view the logs if the USB serial adapter is not available
- A USB to Serial adapter with 3.3V TTL (optional)
- A 5V DC barrel jack

If you're planning to use the serial console follow the docuementation [here](https://www.jetsonhacks.com/2019/04/19/jetson-nano-serial-console/)

First start by downloading the Jetson Nano L4T release.

```bash
curl -SLO https://developer.nvidia.com/embedded/l4t/r32_release_v6.1/t210/jetson-210_linux_r32.6.1_aarch64.tbz2
```

Next we will extract the L4T release and replace the `u-boot` binary with the patched version.

```bash
tar xf jetson-210_linux_r32.6.1_aarch64.tbz2
cd Linux_for_Tegra
crane --platform=linux/arm64 export ghcr.io/talos-systems/u-boot:v0.10.0-alpha.0-11-g5dd08a7 - | tar xf - --strip-components=1 -C bootloader/t210ref/p3450-0000/ jetson_nano/u-boot.bin
```

Next we will flash the firmware to the Jetson Nano SPI flash.
In order to do that we need to put the Jetson Nano into Force Recovery Mode (FRC).
We will use the instructions from [here](https://developer.download.nvidia.com/embedded/L4T/r32_Release_v4.4/r32_Release_v4.4-GMC3/T210/l4t_quick_start_guide.txt)

- Ensure that the Jetson Nano is powered off.
There is no need for the SD card/USB storage/network cable to be connected
- Connect the micro USB cable to the micro USB port on the Jetson Nano, don't plug the other end to the PC yet
- Enable Force Recovery Mode (FRC) by placing a jumper across the FRC pins on the Jetson Nano
  - For board revision *A02*, these are pins `3` and `4` of header `J40`
  - For board revision *B01*, these are pins `9` and `10` of header `J50`
- Place another jumper across `J48` to enable power from the DC jack and connect the Jetson Nano to the DC jack `J25`
- Now connect the other end of the micro USB cable to the PC and remove the jumper wire from the FRC pins

Now the Jetson Nano is in Force Recovery Mode (FRC) and can be confirmed by running the following command

```bash
lsusb | grep -i "nvidia"
```

Now we can move on the flashing the firmware.

```bash
sudo ./flash p3448-0000-max-spi external
```

This will flash the firmware to the Jetson Nano SPI flash and you'll see a lot of output.
If you've connected the serial console you'll also see the progress there.
Once the flashing is done you can disconnect the USB cable and power off the Jetson Nano.

## Download the Image

Download the image and decompress it:

```bash
curl -LO https://github.com/talos-systems/talos/releases/latest/download/metal-jetson_nano-arm64.img.xz
xz -d metal-jetson_nano-arm64.img.xz
```

## Writing the Image

Now `dd` the image to your SD card/USB storage:

```bash
sudo dd if=metal-jetson_nano-arm64.img of=/dev/mmcblk0 conv=fsync bs=4M status=progress
```

| Replace `/dev/mmcblk0` with the name of your SD card/USB storage.

## Bootstrapping the Node

Insert the SD card/USB storage to your board, turn it on and wait for the console to show you the instructions for bootstrapping the node.
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
