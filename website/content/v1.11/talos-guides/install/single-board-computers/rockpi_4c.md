---
title: "Radxa ROCK PI 4C"
description: "Installing Talos on Radxa ROCK PI 4c SBC using raw disk image."
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

The default schematic id for "vanilla" RockPi 4c is `08e72e242b71f42c9db5bed80e8255b2e0d442a372bc09055b79537d9e3ce191`.
Refer to the [Image Factory]({{< relref "../../../learn-more/image-factory" >}}) documentation for more information.

Download the image and decompress it:

```bash
curl -LO https://factory.talos.dev/image/08e72e242b71f42c9db5bed80e8255b2e0d442a372bc09055b79537d9e3ce191/{{< release >}}/metal-arm64.raw.xz
xz -d metal-arm64.raw.xz
```

## Writing the Image

The path to your SD card/eMMC/USB/nVME can be found using `fdisk` on Linux or `diskutil` on macOS.
In this example, we will assume `/dev/mmcblk0`.

Now `dd` the image to your SD card:

```bash
sudo dd if=metal-arm64.raw of=/dev/mmcblk0 conv=fsync bs=4M
```

The user has two options to proceed:

- booting from a SD card or eMMC
- booting from a USB or nVME (requires the RockPi board to have the SPI flash)

### Booting from SD card or eMMC

Insert the SD card into the board, turn it on and proceed to [bootstrapping the node](#bootstrapping-the-node).

### Booting from USB or nVME

This requires the user to flash the RockPi SPI flash with u-boot.

Follow the Radxa docs on [Install on M.2 NVME SSD](https://wiki.radxa.com/Rockpi4/install/NVME)

After these above steps, Talos will boot from the nVME/USB and enter maintenance mode.
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

## Upgrading

For example, to upgrade to the latest version of Talos, you can run:

```bash
talosctl -n <node IP or DNS name> upgrade --image=factory.talos.dev/installer/08e72e242b71f42c9db5bed80e8255b2e0d442a372bc09055b79537d9e3ce191:{{< release >}}
```
