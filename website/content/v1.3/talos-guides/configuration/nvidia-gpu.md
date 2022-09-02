---
title: "NVIDIA GPU (OSS drivers)"
description: "In this guide we'll follow the procedure to support NVIDIA GPU using OSS drivers on Talos."
aliases:
  - ../../guides/nvidia-gpu
---

> Enabling NVIDIA GPU support on Talos is bound by [NVIDIA EULA](https://www.nvidia.com/en-us/drivers/nvidia-license/).
> Talos GPU support has been promoted to **beta**.
> The Talos published NVIDIA OSS drivers are bound to a specific Talos release.
> The extensions versions also needs to be updated when upgrading Talos.

The published versions of the NVIDIA system extensions can be found here:

- [nvidia-open-gpu-kernel-modules](https://github.com/siderolabs/extensions/pkgs/container/nvidia-open-gpu-kernel-modules)
- [nvidia-container-toolkit](https://github.com/siderolabs/extensions/pkgs/container/nvidia-container-toolkit)

## Upgrading Talos and enabling the NVIDIA modules and the system extension

> Make sure to use `talosctl` version {{< release >}} or later

First create a patch yaml `gpu-worker-patch.yaml` to update the machine config similar to below:

```yaml
- op: add
  path: /machine/install/extensions
  value:
    - image: ghcr.io/siderolabs/nvidia-open-gpu-kernel-modules:{{< nvidia_driver_release >}}-{{< release >}}
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
> Make sure the driver version matches for both the `nvidia-open-gpu-kernel-modules` and `nvidia-container-toolkit` extensions.
> The `nvidia-open-gpu-kernel-modules` extension is versioned as `<nvidia-driver-version>-<talos-release-version>` and the `nvidia-container-toolkit` extension is versioned as `<nvidia-driver-version>-<nvidia-container-toolkit-version>`.

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
NODE           NAMESPACE   TYPE              ID                                                                           VERSION   NAME                             VERSION
172.31.41.27   runtime     ExtensionStatus   000.ghcr.io-siderolabs-nvidia-container-toolkit-515.65.01-v1.10.0            1         nvidia-container-toolkit         515.65.01-v1.10.0
172.31.41.27   runtime     ExtensionStatus   000.ghcr.io-siderolabs-nvidia-open-gpu-kernel-modules-515.65.01-v1.2.0       1         nvidia-open-gpu-kernel-modules   515.65.01-v1.2.0
```

```bash
talosctl read /proc/driver/nvidia/version
```

which should produce an output similar to below:

```text
NVRM version: NVIDIA UNIX x86_64 Kernel Module  515.65.01  Wed Mar 16 11:24:05 UTC 2022
GCC version:  gcc version 12.2.0 (GCC)
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

```bash
cat <<EOF | kubectl apply -f -
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
<<EOF
```

The status can be viewed by running:

```bash
kubectl get pods
```

which should produce an output similar to below:

```text
NAME                READY   STATUS      RESTARTS   AGE
gpu-operator-test   0/1     Completed   0          13s
```

```bash
kubectl logs gpu-operator-test
```

which should produce an output similar to below:

```text
[Vector addition of 50000 elements]
Copy input data from the host memory to the CUDA device
CUDA kernel launch with 196 blocks of 256 threads
Copy output data from the CUDA device to the host memory
Test PASSED
Done
```
