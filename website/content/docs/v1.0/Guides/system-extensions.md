---
title: "System Extensions"
---

System extensions allow extending the Talos root filesystem, which enables a variety of features, such as including custom
container runtimes, loading additional firmware, etc.

System extensions are only activated during the installation or upgrade of Talos Linux.
With system extensions installed, the Talos root filesystem is still immutable and read-only.

## Configuration

System extensions are configured in the `.machine.install` section:

```yaml
machine:
  install:
    extensions:
      - image: ghcr.io/talos-systems/gvisor:33f613e
```

During the initial install (e.g. when PXE booting or booting from an ISO), Talos will pull down container images for system extensions,
validate them, and include them into the Talos `initramfs` image.
System extensions will be activated on boot and overlaid on top of the Talos root filesystem.

In order to update the system extensions for a running instance, update `.machine.install.extensions` and upgrade Talos.
(Note: upgrading to the same version of Talos is fine).

> Note: in the next releases of Talos there will be a way to build a Talos image with system extensions included.

## Creating System Extensions

A Talos system extension is a container image with the [specific folder structure](https://github.com/talos-systems/extensions#readme).
System extensions can be built and managed using any tool that produces container images, e.g. `docker build`.

Sidero Labs maintains a [repository of system extensions](https://github.com/talos-systems/extension).

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

See [readme of the gVisor extension](https://github.com/talos-systems/extensions/tree/main/container-runtime/gvisor#readme).
