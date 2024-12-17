---
title: "Kubernetes"
description: "Running Talos Linux as a pod in Kubernetes."
---

Talos Linux can be run as a pod in Kubernetes similar to running Talos in [Docker]({{< relref "../local-platforms/docker" >}}).
This can be used e.g. to run controlplane nodes inside an existing Kubernetes cluster.

Talos Linux running in Kubernetes is not full Talos Linux experience, as it is running in a container using the host's kernel and network stack.
Some operations like upgrades and reboots are not supported.

## Prerequisites

* a running Kubernetes cluster
* a `talos` container image: `ghcr.io/siderolabs/talos:{{< release >}}`

## Machine Configuration

Machine configuration can be generated using [Getting Started]({{< relref "../../../introduction/getting-started" >}}) guide.
Machine install disk will ge ignored, as the install image.
The Talos version will be driven by the container image being used.

The required machine configuration patch to enable using container runtime DNS:

```yaml
machine:
  features:
    hostDNS:
      enabled: true
      forwardKubeDNSToHost: true
```

Talos and Kubernetes API can be exposed using Kubernetes services or load balancers, so they can be accessed from outside the cluster.

## Running Talos Pods

There might be many ways to run Talos in Kubernetes (StatefulSet, Deployment, single Pod), so we will only provide some basic guidance here.

### Container Settings

```yaml
env:
  - name: PLATFORM
    value: container
image: ghcr.io/siderolabs/talos:{{< release >}}
ports:
  - containerPort: 50000
    name: talos-api
    protocol: TCP
  - containerPort: 6443
    name: k8s-api
    protocol: TCP
securityContext:
  privileged: true
  readOnlyRootFilesystem: true
  seccompProfile:
      type: Unconfined
```

### Submitting Initial Machine Configuration

Initial machine configuration can be submitted using `talosctl apply-config --insecure` when the pod is running, or it can be submitted
via an environment variable `USERDATA` with base64-encoded machine configuration.

### Volume Mounts

Three ephemeral mounts are required for `/run`, `/system`, and `/tmp` directories:

```yaml
volumeMounts:
  - mountPath: /run
    name: run
  - mountPath: /system
    name: system
  - mountPath: /tmp
    name: tmp
```

```yaml
volumes:
  - emptyDir: {}
    name: run
  - emptyDir: {}
    name: system
  - emptyDir: {}
    name: tmp
```

Several other mountpoints are required, and they should persist across pod restarts, so one should use `PersistentVolume` for them:

```yaml
volumeMounts:
  - mountPath: /system/state
    name: system-state
  - mountPath: /var
    name: var
  - mountPath: /etc/cni
    name: etc-cni
  - mountPath: /etc/kubernetes
    name: etc-kubernetes
  - mountPath: /usr/libexec/kubernetes
    name: usr-libexec-kubernetes
```
