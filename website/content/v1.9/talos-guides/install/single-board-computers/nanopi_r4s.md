---
title: "Friendlyelec Nano PI R4S"
description: "Installing Talos on a Nano PI R4S SBC using raw disk image."
aliases:
  - ../../../single-board-computers/nanopi_r4s
---

## Prerequisites

You will need

- `talosctl`
- an SD card

Download the latest `talosctl`.

```bash
curl -Lo /usr/local/bin/talosctl https://github.com/siderolabs/talos/releases/download/{{< release >}}/talosctl-$(uname -s | tr "[:upper:]" "[:lower:]")-amd64
chmod +x /usr/local/bin/talosctl
```

## Download the Image

The default schematic id for "vanilla" NanoPi R4S is `5f74a09891d5830f0b36158d3d9ea3b1c9cc019848ace08ff63ba255e38c8da4`.
Refer to the [Image Factory]({{< relref "../../../learn-more/image-factory" >}}) documentation for more information.

Download the image and decompress it:

```bash
curl -LO https://factory.talos.dev/image/5f74a09891d5830f0b36158d3d9ea3b1c9cc019848ace08ff63ba255e38c8da4/{{< release >}}/metal-arm64.raw.xz
xz -d metal-arm64.raw.xz
```

## Writing the Image

The path to your SD card can be found using `fdisk` on Linux or `diskutil` on macOS.
In this example, we will assume `/dev/mmcblk0`.

Now `dd` the image to your SD card:

```bash
sudo dd if=metal-arm64.raw of=/dev/mmcblk0 conv=fsync bs=4M
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

## Upgrading

For example, to upgrade to the latest version of Talos, you can run:

```bash
talosctl -n <node IP or DNS name> upgrade --image=factory.talos.dev/installer/5f74a09891d5830f0b36158d3d9ea3b1c9cc019848ace08ff63ba255e38c8da4:{{< release >}}
```
