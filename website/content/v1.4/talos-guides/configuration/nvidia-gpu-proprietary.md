---
title: "NVIDIA GPU (Proprietary drivers)"
description: "In this guide we'll follow the procedure to support NVIDIA GPU using proprietary drivers on Talos."
aliases:
  - ../../guides/nvidia-gpu-proprietary
---

> Enabling NVIDIA GPU support on Talos is bound by [NVIDIA EULA](https://www.nvidia.com/en-us/drivers/nvidia-license/).

These are the steps to enabling NVIDIA support in Talos.

- Talos pre-installed on a node with NVIDIA GPU installed.
- Building a custom Talos installer image with NVIDIA modules
- Upgrading Talos with the custom installer and enabling NVIDIA modules and the system extension

This requires that the user build and maintain their own Talos installer image.

## Prerequisites

This guide assumes the user has access to a container registry with `push` permissions, docker installed on the build machine and the Talos host has `pull` access to the container registry.

Set the local registry, username and version environment variables:

```bash
export USERNAME=<username>
export REGISTRY=<registry>
export TAG={{< release >}}-nvidia
```

For eg:

```bash
export USERNAME=talos-user
export REGISTRY=ghcr.io
```

> The examples below will use the sample variables set above.
Modify accordingly for your environment.

## Building the installer image

Start by cloning the [pkgs](https://github.com/siderolabs/pkgs) repository.

Now run the following command to build and push custom Talos kernel image and the NVIDIA image with the NVIDIA kernel modules signed by the kernel built along with it.

```bash
make kernel nonfree-kmod-nvidia PLATFORM=linux/amd64 PUSH=true
```

> Replace the platform with `linux/arm64` if building for ARM64

Now we need to create a custom Talos installer image.

Start by creating a `Dockerfile` with the following content:

```Dockerfile
FROM scratch as customization
COPY --from=ghcr.io/talos-user/nonfree-kmod-nvidia:{{< release >}}-nvidia /lib/modules /lib/modules

FROM ghcr.io/siderolabs/installer:{{< release >}}
COPY --from=ghcr.io/talos-user/kernel:{{< release >}}-nvidia /boot/vmlinuz /usr/install/${TARGETARCH}/vmlinuz
```

Now build the image and push it to the registry.

```bash
DOCKER_BUILDKIT=0 docker build --squash --build-arg RM="/lib/modules" -t ghcr.io/talos-user/installer:{{< release >}}-nvidia .
docker push ghcr.io/talos-user/installer:{{< release >}}-nvidia
```

> Note: buildkit has a bug [#816](https://github.com/moby/buildkit/issues/816), to disable it use DOCKER_BUILDKIT=0
> Replace the platform with `linux/arm64` if building for ARM64

## Upgrading Talos and enabling the NVIDIA modules and the system extension

> Make sure to use `talosctl` version {{< release >}} or later

First create a patch yaml `gpu-worker-patch.yaml` to update the machine config similar to below:

```yaml
- op: add
  path: /machine/install/extensions
  value:
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

Now apply the patch to all Talos nodes in the cluster having NVIDIA GPU's installed:

```bash
talosctl patch mc --patch @gpu-worker-patch.yaml
```

Now we can proceed to upgrading Talos with the installer built previously:

```bash
talosctl upgrade --image=ghcr.io/talos-user/installer:{{< release >}}-nvidia
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
