---
title: "Libre Computer Board ALL-H3-CC"
---

## Download the Image

An official image is provided in a release.
Download the compressed image and decompress it:

```bash
curl -LO https://github.com/talos-systems/talos/releases/download/<version>/metal-libretech_all_h3_cc_h5-arm64.img.xz
xz -d metal-libretech_all_h3_cc_h5-arm64.img.xz
```

## Writing the Image

Now `dd` the image your SD card (be sure to update `x` in `mmcblkx`):

```bash
sudo dd if=metal-libretech_all_h3_cc_h5-arm64.img of=/dev/mmcblkx
```

## Bootstrapping the Node

Now insert the SD card, turn on the board, and wait for the console to show you the instructions for bootstrapping the node.
Following the instructions in the console output, generate the configuration files and apply the `init.yaml`:

```bash
talosctl gen config example https://<node IP or DNS name>:6443
talosctl apply-config --insecure --file init.yaml --nodes <node IP or DNS name>
```
