---
title: "NVIDIA Fabric Manager"
description: "In this guide we'll follow the procedure to enable NVIDIA Fabric Manager."
aliases:
  - ../../guides/nvidia-fabricmanager
---

NVIDIA GPUs that have nvlink support (for eg: A100) will need the [nvidia-fabricmanager](https://github.com/siderolabs/extensions/pkgs/container/nvidia-fabricmanager) system extension also enabled in addition to the [NVIDIA drivers]({{< relref "nvidia-gpu" >}}).
For more information on Fabric Manager refer https://docs.nvidia.com/datacenter/tesla/fabric-manager-user-guide/index.html

The published versions of the NVIDIA fabricmanager system extensions is available [here](https://github.com/siderolabs/extensions/pkgs/container/nvidia-fabricmanager)

> The `nvidia-fabricmanager` extension version has to match with the NVIDIA driver version in use.

## Enabling the NVIDIA fabricmanager system extension

Create the [boot assets]({{< relref "../install/boot-assets" >}}) or a custom installer and perform a machine upgrade which include the following system extensions:

```text
ghcr.io/siderolabs/nvidia-open-gpu-kernel-modules-lts:{{< nvidia_driver_release >}}-{{< release >}}
ghcr.io/siderolabs/nvidia-container-toolkit-lts:{{< nvidia_driver_release >}}-{{< nvidia_container_toolkit_release >}}
ghcr.io/siderolabs/nvidia-fabricmanager:{{< nvidia_driver_release >}}
```

Patch the machine configuration to load the required modules:

```yaml
machine:
  kernel:
    modules:
      - name: nvidia
      - name: nvidia_uvm
      - name: nvidia_drm
      - name: nvidia_modeset
  sysctls:
    net.core.bpf_jit_harden: 1
```
