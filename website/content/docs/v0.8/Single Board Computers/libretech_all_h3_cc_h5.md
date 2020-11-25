---
title: "Libre Computer Board ALL-H3-CC"
---

## Generating the Image

Using the Talos installer container, we can generate an image for the libretech_all_h3_cc_h5 by running:

```bash
docker run \
  --rm \
  -v /dev:/dev \
  --privileged \
  ghcr.io/talos-systems/installer:latest image --platform metal --board libretech_all_h3_cc_h5 --tar-to-stdout | tar xz
```

## Writing the Image

Once the image generation is done, extract the raw disk and `dd` it your SD card (be sure to update `x` in `mmcblkx`):

```bash
tar -xvf metal-libretech_all_h3_cc_h5-arm64.tar.gz
sudo dd if=disk.raw of=/dev/mmcblkx
```

## Bootstrapping the Node

Now insert the SD card, turn on the board, and wait for the console to show you the instructions for bootstrapping the node.
Following the instructions in the console output, generate the configuration files and apply the `init.yaml`:

```bash
talosctl gen config libre https://<node IP or DNS name>:6443
talosctl apply-config --insecure --file init.yaml --nodes <node IP or DNS name>
```
