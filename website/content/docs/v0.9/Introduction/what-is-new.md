---
title: What's New in Talos 0.9
weight: 5
---

## Control Plane as Static Pods

Talos now runs Kubernetes control plane as static pods managed via machine configuration.
This change makes bootstrap process much more stable and resilient to failures.
For single control plane node clusters it eliminates bugs with control plane being unavailable after a reboot.
As control plane configuration is managed via Talos API, even if control plane configuration was wrong and
API server is not available, change can be rolled back using `talosctl` to bring the control plane back up.
When upgrading from Talos 0.8, control plane can be [converted](../../guides/converting-control-plane/) to run as static pods.

## ECDSA Certificates and Keys for Kubernetes

Talos now generates uses ECDSA keys for Kubernetes and etcd PKI.
ECDSA keys are much smaller and all PKI operations are much faster (for example, generating certificate from the CA) which
leads to much faster bootstrap and boot times.

## Immediate Machine Configuration Updates

Changes to `.cluster` part of Talos machine configuration can now be [applied immediately](../../guides/editing-machine-configuration) (without a reboot).
This allows for example updating versions of control plane components, adding additional arguments or modifying bootstrap manifests.
Future versions of Talos will expand on that to allow most of the machine configuration to be applied without a reboot.

## Disk Encryption

Talos now supports encryption for `STATE` and `EPHEMERAL` partitions of the system disk.
`STATE` partition holds machine configuration and `EPHEMERAL` partition is mounted as `/var` which stores container runtime
state, configuration files laid on top of Talos read-only immutable root filesystem.
Encryption key in Talos 0.9 is derived from the Node UUID which is unique machine identifier provided by the manufacturer.
Disk encryption is not enabled by default, it needs to be [enabled](../../guides/disk-encryption/) via machine configuration.

## Virtual IP for the Control Plane Endpoint

Talos adds support for Virtual L2 [shared IP](../../guides/vip/) for the control plane: control plane nodes make sure only one of the nodes
adverties shared IP via ARP.
If one of the control plane nodes goes down, another node takes over shared IP.

## Updated Components

Linux: 5.10.1 -> 5.10.19

Kubernetes: 1.20.1 -> 1.20.5

CoreDNS: 1.7.0 -> 1.8.0

etcd: 3.4.14 -> 3.4.15

containerd: 1.4.3 -> 1.4.4
