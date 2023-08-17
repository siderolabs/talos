---
title: "NVIDIA GPU (Proprietary drivers)"
description: "In this guide we'll follow the procedure to support NVIDIA GPU using proprietary drivers on Talos."
aliases:
  - ../../guides/nvidia-gpu-proprietary
---

> Enabling NVIDIA GPU support on Talos is bound by [NVIDIA EULA](https://www.nvidia.com/en-us/drivers/nvidia-license/).
> The Talos published NVIDIA drivers are bound to a specific Talos release.
> The extensions versions also needs to be updated when upgrading Talos.

The published versions of the NVIDIA system extensions can be found here:

- [nonfree-kmod-nvidia](https://github.com/siderolabs/extensions/pkgs/container/nonfree-kmod-nvidia)
- [nvidia-container-toolkit](https://github.com/siderolabs/extensions/pkgs/container/nvidia-container-toolkit)

> To build a NVIDIA driver version not published by SideroLabs jump to [Building the NVIDIA extensions](#building-the-nvidia-extensions) and then use those in the steps below instead of the ones published by SideroLabs

## Upgrading Talos and enabling the NVIDIA modules and the system extension

> Make sure to use `talosctl` version {{< release >}} or later

First create a patch yaml `gpu-worker-patch.yaml` to update the machine config similar to below:

```yaml
- op: add
  path: /machine/install/extensions
  value:
    - image: ghcr.io/siderolabs/nonfree-kmod-nvidia:{{< nvidia_driver_release >}}-{{< release >}}
    - image: ghcr.io/siderolabs/nvidia-container-toolkit:{{< nvidia_driver_release >}}-{{< nvidia_container_toolkit_release >}}
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

> Update the driver version and Talos release in the above patch yaml from the published versions if there is a newer one available.
> Make sure the driver version matches for both the `nonfree-kmod-nvidia` and `nvidia-container-toolkit` extensions.
> The `nonfree-kmod-nvidia` extension is versioned as `<nvidia-driver-version>-<talos-release-version>` and the `nvidia-container-toolkit` extension is versioned as `<nvidia-driver-version>-<nvidia-container-toolkit-version>`.

Now apply the patch to all Talos nodes in the cluster having NVIDIA GPU's installed:

```bash
talosctl patch mc --patch @gpu-worker-patch.yaml
```

Now we can proceed to upgrading Talos to the same version to enable the system extension:

```bash
talosctl upgrade --image=ghcr.io/siderolabs/installer:{{< release >}}
```

Once the node reboots, the NVIDIA modules should be loaded and the system extension should be installed.

This can be confirmed by running:

```bash
talosctl read /proc/modules
```

which should produce an output similar to below:

```text
nvidia_uvm 1146880 - - Live 0xffffffffc2733000 (PO)
nvidia_drm 69632 - - Live 0xffffffffc2721000 (PO)
nvidia_modeset 1142784 - - Live 0xffffffffc25ea000 (PO)
nvidia 39047168 - - Live 0xffffffffc00ac000 (PO)
```

```bash
talosctl get extensions
```

which should produce an output similar to below:

```text
NODE           NAMESPACE   TYPE              ID                                                                 VERSION   NAME                       VERSION
172.31.41.27   runtime     ExtensionStatus   000.ghcr.io-frezbo-nvidia-container-toolkit-510.60.02-v1.9.0       1         nvidia-container-toolkit   510.60.02-v1.9.0
```

```bash
talosctl read /proc/driver/nvidia/version
```

which should produce an output similar to below:

```text
NVRM version: NVIDIA UNIX x86_64 Kernel Module  510.60.02  Wed Mar 16 11:24:05 UTC 2022
GCC version:  gcc version 11.2.0 (GCC)
```

## Deploying NVIDIA device plugin

First we need to create the `RuntimeClass`

Apply the following manifest to create a runtime class that uses the extension:

```yaml
---
apiVersion: node.k8s.io/v1
kind: RuntimeClass
metadata:
  name: nvidia
handler: nvidia
```

Install the NVIDIA device plugin:

```bash
helm repo add nvdp https://nvidia.github.io/k8s-device-plugin
helm repo update
helm install nvidia-device-plugin nvdp/nvidia-device-plugin --version=0.13.0 --set=runtimeClassName=nvidia
```

## (Optional) Setting the default runtime class as `nvidia`

> Do note that this will set the default runtime class to `nvidia` for all pods scheduled on the node.

Create a patch yaml `nvidia-default-runtimeclass.yaml` to update the machine config similar to below:

```yaml
- op: add
  path: /machine/files
  value:
    - content: |
        [plugins]
          [plugins."io.containerd.grpc.v1.cri"]
            [plugins."io.containerd.grpc.v1.cri".containerd]
              default_runtime_name = "nvidia"
      path: /etc/cri/conf.d/20-customization.part
      op: create
```

Now apply the patch to all Talos nodes in the cluster having NVIDIA GPU's installed:

```bash
talosctl patch mc --patch @nvidia-default-runtimeclass.yaml
```

### Testing the runtime class

> Note the `spec.runtimeClassName` being explicitly set to `nvidia` in the pod spec.

Run the following command to test the runtime class:

```bash
kubectl run \
  nvidia-test \
  --restart=Never \
  -ti --rm \
  --image nvcr.io/nvidia/cuda:12.1.0-base-ubuntu22.04 \
  --overrides '{"spec": {"runtimeClassName": "nvidia"}}' \
  nvidia-smi
```

## Building the NVIDIA extensions

If you want to build the NVIDIA extensions yourself instead of using the extensions
published by SideroLabs start by cloning the `release-1.5` branch [extensions](https://github.com/siderolabs/extensions) repository.

```bash
git clone --depth=1 --branch=release-1.5 https://github.com/siderolabs/extensions.git
```

Lookup the version of [pkgs](https://github.com/siderolabs/pkgs) used for the particular Talos version at `https://github.com/siderolabs/talos/blob/<talos-version>/pkg/machinery/gendata/data/pkgs`.

Now run the following command to build and push custom NVIDIA extension.

```bash
make nonfree-kmod-nvidia PKGS=<pkgs-version-looked-up-above> PLATFORM=linux/amd64 PUSH=true
```

> Replace the platform with `linux/arm64` if building for ARM64.
> To change the NVIDIA driver version modify the build argument in
> `nvidia-gpu/nonfree/kmod-nvidia/vars.yaml` accordingly.
> Make sure to use `talosctl` version {{< release >}} or later
