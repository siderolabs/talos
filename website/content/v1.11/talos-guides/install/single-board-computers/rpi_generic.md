---
title: "Raspberry Pi Series"
description: "Installing Talos on Raspberry Pi SBC's using raw disk image."
aliases:
  - ../../../single-board-computers/rpi_generic
---

Talos disk image for the Raspberry Pi generic should in theory work for the boards supported by [u-boot](https://github.com/u-boot/u-boot/blob/master/doc/board/broadcom/raspberrypi.rst#64-bit) `rpi_arm64_defconfig`.
This has only been officialy tested on the Raspberry Pi 4 and community tested on one variant of the Compute Module 4 using Super 6C boards.
If you have tested this on other Raspberry Pi boards, please let us know.

## Video Walkthrough

To see a live demo of this writeup, see the video below:

<iframe width="560" height="315" src="https://www.youtube.com/embed/aHu1lFir7UU" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

## Prerequisites

You will need

- `talosctl`
- an SD card

Download the latest `talosctl`.

```bash
curl -sL 'https://www.talos.dev/install' | bash
```

## Updating the EEPROM

Use [Raspberry Pi Imager](https://www.raspberrypi.com/software/) to write an EEPROM update image to a spare SD card.
Select Misc utility images under the Operating System tab.

Remove the SD card from your local machine and insert it into the Raspberry Pi.
Power the Raspberry Pi on, and wait at least 10 seconds.
If successful, the green LED light will blink rapidly (forever), otherwise an error pattern will be displayed.
If an HDMI display is attached to the port closest to the power/USB-C port,
the screen will display green for success or red if a failure occurs.
Power off the Raspberry Pi and remove the SD card from it.

> Note: Updating the bootloader only needs to be done once.

## Download the Image

> Note: if you need to enable Broadcom VideoCore GPU support, generate a new image from the [Image Factory]({{< relref "../../../learn-more/image-factory" >}}) with the correct [config.txt](#configtxt-information) configuration and `vc4` system extension.
> More information can be found under the [Image Factory Example](#example-raspberry-pi-generic-with-broadcom-videocore-gpu-support-with-image-factory) below.

The default schematic id for "vanilla" Raspberry Pi generic image is `ee21ef4a5ef808a9b7484cc0dda0f25075021691c8c09a276591eedb638ea1f9`.Refer to the [Image Factory]({{< relref "../../../learn-more/image-factory" >}}) documentation for more information.

Download the image and decompress it:

```bash
curl -LO https://factory.talos.dev/image/ee21ef4a5ef808a9b7484cc0dda0f25075021691c8c09a276591eedb638ea1f9/{{< release >}}/metal-arm64.raw.xz
xz -d metal-arm64.raw.xz
```

## Writing the Image

Now `dd` the image to your SD card:

```bash
sudo dd if=metal-arm64.raw of=/dev/mmcblk0 conv=fsync bs=4M
```

## Bootstrapping the Node

Insert the SD card to your board, turn it on and wait for the console to show you the instructions for bootstrapping the node.
Following the instructions in the console output to connect to the interactive installer:

> Note: Add the [vc4 System Extension](https://github.com/siderolabs/extensions/pkgs/container/vc4) for V3D/VC4 Broadcom VideoCore GPU support.

```bash
talosctl apply-config --insecure --mode=interactive --nodes <node IP or DNS name>
```

Once the interactive installation is applied, the cluster will form and you can then use `kubectl`.

> Note: if you have an HDMI display attached and it shows only a rainbow splash,
> please use the other HDMI port, the one closest to the power/USB-C port.

## Retrieve the `kubeconfig`

Retrieve the admin `kubeconfig` by running:

```bash
talosctl kubeconfig
```

## Upgrading

For example, to upgrade to the latest version of Talos, you can run:

```bash
talosctl -n <node IP or DNS name> upgrade --image=factory.talos.dev/installer/ee21ef4a5ef808a9b7484cc0dda0f25075021691c8c09a276591eedb638ea1f9:{{< release >}}
```

### Example: Raspberry Pi generic with Broadcom VideoCore GPU support with Image Factory

Let's assume we want to boot Talos on a Raspberry Pi with the `vc4` system extension for V3D/VC4 Broadcom VideoCore GPU support.

First, let's create the schematic file `rpi_generic.yaml`:

#### Schematic example with `vc4` system extension

```yaml
# rpi_generic.yaml
overlay:
  name: rpi_generic
  image: siderolabs/sbc-raspberrypi
  options:
    configTxt: |
      gpu_mem=128
      kernel=u-boot.bin
      arm_64bit=1
      arm_boost=1
      enable_uart=1
      dtoverlay=disable-bt
      dtoverlay=disable-wifi
      avoid_warnings=2
      dtoverlay=vc4-kms-v3d,noaudio
customization:
  systemExtensions:
    officialExtensions:
      - siderolabs/vc4
```

> The schematic doesn't contain any system extension or overlay versions, Image Factory will pick the correct version matching Talos Linux release.

And now we can upload the schematic to the Image Factory to retrieve its ID:

```shell
$ curl -X POST --data-binary @rpi_generic.yaml https://factory.talos.dev/schematics
{"id":"0db665edfda21c70194e7ca660955425d16cec2aa58ff031e2abf72b7c328585"}
```

The returned schematic ID `0db665edfda21c70194e7ca660955425d16cec2aa58ff031e2abf72b7c328585` we will use to generate the boot assets.

> The schematic ID is based on the schematic contents, so uploading the same schematic will return the same ID.

Now we can download the metal arm64 image:

- https://factory.talos.dev/image/0db665edfda21c70194e7ca660955425d16cec2aa58ff031e2abf72b7c328585/{{< release >}}/metal-arm64.raw.xz (download it and burn to a boot media)

> The Image Factory URL contains both schematic ID and Talos version, and both can be changed to generate different boot assets.

Once installed, the machine can be upgraded to a new version of Talos by referencing new installer image:

```shell
talosctl upgrade --image factory.talos.dev/installer/0db665edfda21c70194e7ca660955425d16cec2aa58ff031e2abf72b7c328585:<new_version>
```

Same way upgrade process can be used to transition to a new set of system extensions: generate new schematic with the new set of system extensions, and upgrade the machine to the new schematic ID:

```shell
talosctl upgrade --image factory.talos.dev/installer/<new_schematic_id>:{{< release >}}
```

### Example: Raspberry Pi generic with Broadcom VideoCore GPU support with Imager

Let's assume we want to boot Talos on Raspberry Pi with `rpi_generic` overlay and the `vc4` system extension for Broadcom VideoCore GPU support.

First, let's lookup extension images for `vc4` in the [extensions repository](https://github.com/siderolabs/extensions):

```shell
$ crane export ghcr.io/siderolabs/extensions:{{< release >}} | tar x -O image-digests | grep -E 'vc4'
ghcr.io/siderolabs/vc4:v0.1.4@sha256:548b2b121611424f6b1b6cfb72a1669421ffaf2f1560911c324a546c7cee655e
```

Next we'll lookup the overlay image for `rpi_generic` in the [overlays repository](https://github.com/siderolabs/overlays):

```shell
$ crane export ghcr.io/siderolabs/overlays:{{< release >}} | tar x -O overlays.yaml | yq '.overlays[] | select(.name=="rpi_generic")'
name: rpi_generic
image: ghcr.io/siderolabs/sbc-raspberrypi:v0.1.0
digest: sha256:849ace01b9af514d817b05a9c5963a35202e09a4807d12f8a3ea83657c76c863
```

Now we can generate the metal image with the following command:

```shell
$ docker run --rm -t -v $PWD/_out:/out -v /dev:/dev --privileged ghcr.io/siderolabs/imager:{{< release >}} rpi_generic \
  --arch arm64 \
  --overlay-image ghcr.io/siderolabs/sbc-raspberrypi:v0.1.0@sha256:849ace01b9af514d817b05a9c5963a35202e09a4807d12f8a3ea83657c76c863 \
  --overlay-name=rpi_generic \
  --overlay-option="configTxt=$(cat <<EOF
gpu_mem=128
kernel=u-boot.bin
arm_64bit=1
arm_boost=1
enable_uart=1
dtoverlay=disable-bt
dtoverlay=disable-wifi
avoid_warnings=2
dtoverlay=vc4-kms-v3d,noaudio
EOF
)" \
  --system-extension-image ghcr.io/siderolabs/vc4:v0.1.4@sha256:548b2b121611424f6b1b6cfb72a1669421ffaf2f1560911c324a546c7cee655e
profile ready:
arch: arm64
platform: metal
secureboot: false
version: {{< release >}}
input:
  kernel:
    path: /usr/install/arm64/vmlinuz
  initramfs:
    path: /usr/install/arm64/initramfs.xz
  baseInstaller:
    imageRef: ghcr.io/siderolabs/installer:{{< release >}}
  systemExtensions:
    - imageRef: ghcr.io/siderolabs/vc4:v0.1.4@sha256:a68c268d40694b7b93c8ac65d6b99892a6152a2ee23fdbffceb59094cc3047fc
overlay:
  name: rpi_generic
  image:
    imageRef: ghcr.io/siderolabs/sbc-raspberrypi:v0.1.0-alpha.1@sha256:849ace01b9af514d817b05a9c5963a35202e09a4807d12f8a3ea83657c76c863
  options:
    configTxt: |-
      gpu_mem=128
      kernel=u-boot.bin
      arm_64bit=1
      arm_boost=1
      enable_uart=1
      dtoverlay=disable-bt
      dtoverlay=disable-wifi
      avoid_warnings=2
      dtoverlay=vc4-kms-v3d,noaudio
output:
  kind: image
  imageOptions:
    diskSize: 1306525696
    diskFormat: raw
  outFormat: .xz
initramfs ready
kernel command line: talos.platform=metal console=tty0 console=ttyAMA0,115200 sysctl.kernel.kexec_load_disabled=1 talos.dashboard.disabled=1 init_on_alloc=1 slab_nomerge pti=on consoleblank=0 nvme_core.io_timeout=4294967295 printk.devkmsg=on
disk image ready
output asset path: /out/metal-arm64.raw
compression done: /out/metal-arm64.raw.xz
```

Now the `_out/metal-arm64.raw.xz` is the compressed disk image which can be written to a boot media.

As the next step, we should generate a custom `installer` image which contains all required system extensions (kernel args can't be specified with the installer image, but they are set in the machine configuration):

```shell
$ docker run --rm -t -v $PWD/_out:/out ghcr.io/siderolabs/imager:{{< release >}} installer \
  --arch arm64 \
  --overlay-image ghcr.io/siderolabs/sbc-raspberrypi:v0.1.0@sha256:849ace01b9af514d817b05a9c5963a35202e09a4807d12f8a3ea83657c76c863 \
  --overlay-name=rpi_generic \
  --overlay-option="configTxt=$(cat <<EOF
gpu_mem=128
kernel=u-boot.bin
arm_64bit=1
arm_boost=1
enable_uart=1
dtoverlay=disable-bt
dtoverlay=disable-wifi
avoid_warnings=2
dtoverlay=vc4-kms-v3d,noaudio
EOF
)" \
  --system-extension-image ghcr.io/siderolabs/vc4:v0.1.4@sha256:548b2b121611424f6b1b6cfb72a1669421ffaf2f1560911c324a546c7cee655e
...
output asset path: /out/metal-arm64-installer.tar
```

The `installer` container image should be pushed to the container registry:

```shell
crane push _out/metal-arm64-installer.tar ghcr.io/<username></username>/installer:{{< release >}}
```

Now we can use the customized `installer` image to install Talos on Raspberry Pi.

When it's time to upgrade a machine, a new `installer` image can be generated using the new version of `imager`, and updating the system extension and overlay images to the matching versions.
The custom `installer` image can now be used to upgrade Talos machine.

## config.txt Information

Refer to the default [config.txt](https://github.com/siderolabs/sbc-raspberrypi/blob/main/installers/rpi_generic/src/config.txt) file used by the [sbc-raspberrypi](https://github.com/siderolabs/sbc-raspberrypi) overlay.

### Configure the `config.txt` file for usage with the `vc4` system extension

```ini
...
gpu_mem=128 # <== Add or edit this line
...
hdmi_safe:0=1 # <== Remove this line
hdmi_safe:1=1 # <== Remove this line
...
avoid_warnings=2 # <== Add this line
dtoverlay=vc4-kms-v3d,noaudio # <== Add this line
...
```

## Troubleshooting

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
