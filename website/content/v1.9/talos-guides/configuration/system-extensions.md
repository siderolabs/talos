---
title: "System Extensions"
description: "Customizing the Talos Linux immutable root file system."
aliases:
  - ../../guides/system-extensions
  - ../../advanced/customizing-the-root-filesystem
---

System extensions allow extending the Talos root filesystem, which enables a variety of features, such as including custom
container runtimes, loading additional firmware, etc.

System extensions are only activated during the installation or upgrade of Talos Linux.
With system extensions installed, the Talos root filesystem is still immutable and read-only.

## Installing System Extensions

> Note: the way to install system extensions in the `.machine.install` section of the machine configuration is now deprecated.

Starting with Talos v1.5.0, Talos supports generation of boot media with system extensions included, this removes the need to rebuild
the `initramfs.xz` on the machine itself during the installation or upgrade.

There are two kinds of boot assets that Talos can generate:

* initial boot assets (ISO, PXE, etc.) that are used to boot the machine
* disk images that have Talos pre-installed
* `installer` container images that can be used to install or upgrade Talos on a machine (installation happens when booted from ISO or PXE)

Depending on the nature of the system extension (e.g. network device driver or `containerd` plugin), it may be necessary to include the extension in
both initial boot assets and disk images/`installer`, or just the `installer`.

The process of generating boot assets with extensions included is described in the [boot assets guide]({{< relref "../install/boot-assets" >}}).

### Example: Booting from an ISO

Let's assume NVIDIA extension is required on a bare metal machine which is going to be booted from an ISO.
As NVIDIA extension is not required for the initial boot and install step, it is sufficient to include the extension in the `installer` image only.

1. Use a generic Talos ISO to boot the machine.
2. Prepare a custom `installer` container image with NVIDIA extension included, push the image to a registry.
3. Ensure that machine configuration field `.machine.install.image` points to the custom `installer` image.
4. Boot the machine using the ISO, apply the machine configuration.
5. Talos pulls a custom installer image from the registry (containing NVIDIA extension), installs Talos on the machine, and reboots.

When it's time to upgrade Talos, generate a custom `installer` container for a new version of Talos, push it to a registry, and perform upgrade
pointing to the custom `installer` image.

### Example: Disk Image

Let's assume NVIDIA extension is required on AWS VM.

1. Prepare an AWS disk image with NVIDIA extension included.
2. Upload the image to AWS, register it as an AMI.
3. Use the AMI to launch a VM.
4. Talos boots with NVIDIA extension included.

When it's time to upgrade Talos, either repeat steps 1-4 to replace the VM with a new AMI, or
like in the previous example, generate a custom `installer` and use it to upgrade Talos in-place.

## Authoring System Extensions

A Talos system extension is a container image with the [specific folder structure](https://github.com/siderolabs/extensions#readme).
System extensions can be built and managed using any tool that produces container images, e.g. `docker build`.

Sidero Labs maintains a [repository of system extensions](https://github.com/siderolabs/extensions).

## Resource Definitions

Use `talosctl get extensions` to get a list of system extensions:

```bash
$ talosctl get extensions
NODE         NAMESPACE   TYPE              ID                                              VERSION   NAME          VERSION
172.20.0.2   runtime     ExtensionStatus   000.ghcr.io-talos-systems-gvisor-54b831d        1         gvisor        20220117.0-v1.0.0
172.20.0.2   runtime     ExtensionStatus   001.ghcr.io-talos-systems-intel-ucode-54b831d   1         intel-ucode   microcode-20210608-v1.0.0
```

Use YAML or JSON format to see additional details about the extension:

```bash
$ talosctl -n 172.20.0.2 get extensions 001.ghcr.io-talos-systems-intel-ucode-54b831d -o yaml
node: 172.20.0.2
metadata:
    namespace: runtime
    type: ExtensionStatuses.runtime.talos.dev
    id: 001.ghcr.io-talos-systems-intel-ucode-54b831d
    version: 1
    owner: runtime.ExtensionStatusController
    phase: running
    created: 2022-02-10T18:25:04Z
    updated: 2022-02-10T18:25:04Z
spec:
    image: 001.ghcr.io-talos-systems-intel-ucode-54b831d.sqsh
    metadata:
        name: intel-ucode
        version: microcode-20210608-v1.0.0
        author: Spencer Smith
        description: |
            This system extension provides Intel microcode binaries.
        compatibility:
            talos:
                version: '>= v1.0.0'
```

## Example: gVisor

See [readme of the gVisor extension](https://github.com/siderolabs/extensions/tree/main/container-runtime/gvisor#readme).
