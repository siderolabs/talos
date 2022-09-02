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

## Upgrading Talos and enabling the NVIDIA fabricmanager system extension

In addition to the patch defined in the [NVIDIA drivers]({{< relref "nvidia-gpu" >}}#upgrading-talos-and-enabling-the-nvidia-modules-and-the-system-extension) guide, we need to add the `nvidia-fabricmanager` system extension to the patch yaml `gpu-worker-patch.yaml`:

```yaml
- op: add
  path: /machine/install/extensions
  value:
    - image: ghcr.io/siderolabs/nvidia-open-gpu-kernel-modules:{{< nvidia_driver_release >}}-{{< release >}}
    - image: ghcr.io/siderolabs/nvidia-container-toolkit:{{< nvidia_driver_release >}}-{{< nvidia_container_toolkit_release >}}
    - image: ghcr.io/siderolabs/nvidia-fabricmanager:{{< nvidia_driver_release >}}
- op: add
  path: /machine/kernel
  value:
    modules:
      - name: nvidia
      - name: nvidia_uvm
      - name: nvidia_drm
      - name: nvidia_modeset
- op: add
  path: /machine/sysctls
  value:
    net.core.bpf_jit_harden: 1
```
