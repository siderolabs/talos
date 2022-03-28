---
title: "NVIDIA GPU"
description: "In this guide we'll follow the procedure to support NVIDIA GPU on Talos."
---

> Enabling NVIDIA GPU support on Talos is bound by [NVIDIA EULA](https://www.nvidia.com/en-us/drivers/nvidia-license/)
> Talos GPU support is an **alpha** feature.

There are three steps to enabling NVIDIA support in Talos.

- Building a custom Talos installer image with NVIDIA modules
- Building NVIDIA container toolkit system extension which allows to register a custom runtime with containerd
- Upgrading Talos with the custom installer and enabling NVIDIA modules and the system extension

Both these components require that the user build and maintain their own Talos installer image and the NVIDIA container toolkit [Talos System Extension](../system-extensions).

## Prerequisites

This guide assumes the user has access to a container registry with push permissions, docker installer on the build machine and the Talos host has access to the registry.

Set the local registry and username environment variables:

```bash
export USERNAME=<username>
export REGISTRY=<registry>
export TAG=<custom installer image tag>
```

## Building the installer image

Start by cloning the [pkgs](https://github.com/siderolabs/pkgs) repository.

Now run the following command to build and push custom Talos kernel image and the NVIDIA image with the NVIDIA kernel modules signed by the kernel built along with it.

Replace the platform with `linux/arm64` if building for ARM64

```bash
make kernel nonfree-kmod-nvidia PLATFORM=linux/amd64 PUSH=true
```

Now we need to create a custom Talos installer image.

Start by creating a `Dockerfile` with the following content:

```Dockerfile
FROM scratch as customization
COPY --from=<registry>/<username>/nonfree-kmod-nvidia:<custom installer image tag> /lib/modules /lib/modules

FROM ghcr.io/siderolabs/installer:{{< release >}}
COPY --from=<registry>/<username>/kernel:<custom installer image tag> /boot/vmlinuz /usr/install/${TARGETARCH}/vmlinuz
```

Now built the image and push it to the registry.

```bash
DOCKER_BUILDKIT=0 docker build --squash --build-arg RM="/lib/modules" -t <registry>/<username>/installer:<custom installer image tag> .
docker push <registry>/<username>/installer:<custom installer image tag>
```

> Note: buildkit has a bug [#816](https://github.com/moby/buildkit/issues/816), to disable it use DOCKER_BUILDKIT=0

## Building the system extension

Start by cloning the [extensions](https://github.com/siderolabs/extensions) repository.

Now run the following command to build and push the system extension.

Replace the platform with `linux/arm64` if building for ARM64

```bash
make nvidia-container-toolkit PLATFORM=linux/amd64 PUSH=true
```

## Upgrading Talos and enabling the NVIDIA modules and the system extension

> Make sure to use `talosctl` version {{< release >}} or later

First create a patch yaml `gpu-worker-patch.yaml` to update the machine config similar to below:

```yaml
- op: add
  path: /machine/install/extensions
  value:
    - image: <registry>/<username>/nvidia-container-toolkit:<custom installer image tag>
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

Now apply the patch to the machine config:

```bash
talosctl patch mc --patch @gpu-worker-patch.yaml
```

Now we can proceed to upgrading Talos with the installer built previously:

```bash
talosctl upgrade --image=<registry>/<username>/installer:<custom installer image tag>
```

Once the node reboots, the NVIDIA modules should be loaded and the system extension should be installed.

This can be confirmed by running:

```bash
❯ talosctl read /proc/modules
nvidia_uvm 1146880 - - Live 0xffffffffc2733000 (PO)
nvidia_drm 69632 - - Live 0xffffffffc2721000 (PO)
nvidia_modeset 1142784 - - Live 0xffffffffc25ea000 (PO)
nvidia 39047168 - - Live 0xffffffffc00ac000 (PO)
```

```bash
❯ talosctl get extensions
NODE           NAMESPACE   TYPE              ID                                                                 VERSION   NAME                       VERSION
172.31.41.27   runtime     ExtensionStatus   000.ghcr.io-frezbo-nvidia-container-toolkit-v1.0.0-beta.3-nvidia   1         nvidia-container-toolkit   510.60.02-v1.9.0
```

```bash
❯ talosctl read /proc/driver/nvidia/version
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
helm install nvidia-device-plugin nvdp/nvidia-device-plugin --version=0.11.0 --set=runtimeClassName=nvidia
```

Apply the following manifest to run CUDA pod via nvidia runtime:

```yaml
---
apiVersion: v1
kind: Pod
metadata:
  name: gpu-operator-test
spec:
  restartPolicy: OnFailure
  runtimeClassName: nvidia
  containers:
  - name: cuda-vector-add
    image: "nvidia/samples:vectoradd-cuda11.6.0"
    resources:
      limits:
         nvidia.com/gpu: 1
```

The status can be viewed by running:

```bash
❯ kubectl get pods
NAME                READY   STATUS      RESTARTS   AGE
gpu-operator-test   0/1     Completed   0          13s
```

```bash
❯ kubectl logs gpu-operator-test
[Vector addition of 50000 elements]
Copy input data from the host memory to the CUDA device
CUDA kernel launch with 196 blocks of 256 threads
Copy output data from the CUDA device to the host memory
Test PASSED
Done
```
