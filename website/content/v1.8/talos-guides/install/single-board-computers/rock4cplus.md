---
title: "Radxa ROCK 4C Plus"
description: "Installing Talos on Radxa ROCK 4c Plus SBC using raw disk image."
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

The default schematic id for "vanilla" Rock 4c Plus is `ed7091ab924ef1406dadc4623c90f245868f03d262764ddc2c22c8a19eb37c1c`.
Refer to the [Image Factory]({{< relref "../../../learn-more/image-factory" >}}) documentation for more information.

Download the image and decompress it:

```bash
curl -LO https://factory.talos.dev/image/ed7091ab924ef1406dadc4623c90f245868f03d262764ddc2c22c8a19eb37c1c/{{< release >}}/metal-arm64.raw.xz
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

### Booting from SD card or eMMC

Insert the SD card into the board, turn it on and proceed to [bootstrapping the node](#bootstrapping-the-node).

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
talosctl -n <node IP or DNS name> upgrade --image=factory.talos.dev/installer/ed7091ab924ef1406dadc4623c90f245868f03d262764ddc2c22c8a19eb37c1c:{{< release >}}
```
