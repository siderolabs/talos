---
title: "Turing RK1"
description: "Installing Talos on Turing RK1 SOM using raw disk image."
aliases: 
  - ../../../single-board-computers/turing_rk1
---

## Prerequisites

Before you start, ensure you have:

- `talosctl`
- `tpi` from [github](https://github.com/turing-machines/tpi/releases)
- [crane CLI](https://github.com/google/go-containerregistry/releases)

Download the latest `talosctl`.

```bash
curl -Lo /usr/local/bin/talosctl https://github.com/siderolabs/talos/releases/download/{{< release >}}/talosctl-$(uname -s | tr "[:upper:]" "[:lower:]")-amd64
chmod +x /usr/local/bin/talosctl
```

## Download the Image

Go to `https://factory.talos.dev` select `Single Board Computers`, select the version and select `Turing RK1` from the options.
Choose your desired extensions and fill in the kernel command line arguments if needed.

Download the disk image and decompress it:

```bash
curl -LO https://factory.talos.dev/image/[uuid]/v1.9.0/metal-arm64.raw.xz
xz -d metal-arm64.raw.xz
```

## Boot options

You can boot Talos from:

1. booting from eMMC
2. booting from a USB or NVMe (requires a spi image on the eMMC)

### Booting from eMMC

Flash the image to the eMMC and power on the node: (or use the WebUI of the Turing Pi 2)

```bash
tpi flash -n <NODENUMBER> -i metal-arm64.raw
tpi power on -n <NODENUMBER> 
```

Proceed to [bootstrapping the node](#bootstrapping-the-node).

### Booting from USB or NVMe

#### Requirements

To boot from USB or NVMe, flash a u-boot SPI image (part of the SBC overlay) to the eMMC.

#### Steps

Skip step 1 if you already installed your NVMe drive.

1. If you have a USB to NVMe adapter, write Talos image to the USB drive:

     ```bash
     sudo dd if=metal-arm64.raw of=/dev/sda
     ```
  
2. Install the NVMe drive in the Turing Pi 2 board.

    If the NVMe drive is/was already installed:

    - Flash the Turing RK1 variant of [Ubuntu](https://docs.turingpi.com/docs/turing-rk1-flashing-os) to the eMMC.
    - Boot into the Ubuntu image and write the Talos image directly to the NVMe drive:
  
      ```bash
      sudo dd if=metal-arm64.raw of=/dev/nvme0n1
      ```

3. Find the latest `sbc-rockchip` overlay, download and extract the SBC overlay image:

    - Find the latest release tag of the [sbc-rockchip repo](https://github.com/siderolabs/sbc-rockchip/releases).
    - Download the sbc overlay image and extract the SPI image:

      ```bash
      crane --platform=linux/arm64 export ghcr.io/siderolabs/sbc-rockchip:<releasetag> | tar x --strip-components=4 artifacts/arm64/u-boot/turingrk1/u-boot-rockchip-spi.bin
      ```

4. Flash the eMMC with the Talos raw image (even if Talos was previously installed): (or use the WebUI of the Turing Pi 2)

    ```bash
    tpi flash -n <NODENUMBER> -i metal-turing_rk1-arm64.raw
    ```

5. Flash the SPI image to set the boot order and remove unnecessary partitions: (or use the WebUI of the Turing Pi 2)

    ```bash
    tpi flash -n <NODENUMBER> -i u-boot-rockchip-spi.bin
    tpi power on -n <NODENUMBER>
    ```

Talos will now boot from the NVMe/USB and enter maintenance mode.

## Bootstrapping the Node

To monitor boot messages, run: (repeat)

```sh
tpi uart -n <NODENUMBER> get
```

Wait until instructions for bootstrapping appear.
Follow the UART instructions to connect to the interactive installer:

```bash
talosctl apply-config --insecure --mode=interactive --nodes <node IP or DNS name>
```

Alternatively, generate and apply a configuration:

```bash
talosctl gen config
talosctl apply-config --insecure --nodes <node IP or DNS name> -f <worker/controlplane>.yaml
```

Copy your `talosconfig` to `~/.talos/config` and fill in the `node` field with the IP address of the node and endpoints.

Once applied, the cluster will form, and you can use `kubectl`.

## Retrieve the `kubeconfig`

Retrieve the admin `kubeconfig` by running:

```bash
talosctl kubeconfig
```
