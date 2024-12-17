---
title: "Pine64"
description: "Installing Talos on a Pine64 SBC using raw disk image."
aliases:
  - ../../../single-board-computers/pine64
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

The default schematic id for "vanilla" Pine64 is `185431e0f0bf34c983c6f47f4c6d3703aa2f02cd202ca013216fd71ffc34e175`.
Refer to the [Image Factory]({{< relref "../../../learn-more/image-factory" >}}) documentation for more information.

Download the image and decompress it:

```bash
curl -LO https://factory.talos.dev/image/185431e0f0bf34c983c6f47f4c6d3703aa2f02cd202ca013216fd71ffc34e175/{{< release >}}/metal-arm64.raw.xz
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
talosctl -n <node IP or DNS name> upgrade --image=factory.talos.dev/installer/185431e0f0bf34c983c6f47f4c6d3703aa2f02cd202ca013216fd71ffc34e175:{{< release >}}
```
