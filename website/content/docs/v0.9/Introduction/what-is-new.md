---
title: What's New in Talos 0.9
weight: 5
---

## Control Plane as Static Pods

Talos now runs the Kubernetes control plane as static pods managed via machine configuration.
This change makes the bootstrap process much more stable and resilient to failures.
For single control plane node clusters it eliminates bugs with the control plane being unavailable after a reboot.
As the control plane configuration is managed via the Talos API, even if the control plane configuration was wrong and
the API server is not available, the change can be rolled back using `talosctl` to bring the control plane back up.
When upgrading from Talos 0.8, control plane can be [converted](../../guides/converting-control-plane/) to run as static pods.

## ECDSA Certificates and Keys for Kubernetes

Talos now generates uses ECDSA keys for Kubernetes and etcd PKI.
ECDSA keys are much smaller than RSA keys and all PKI operations are much faster (for example, generating a certificate from the CA) which
leads to much faster bootstrap and boot times.

## Immediate Machine Configuration Updates

Changes to the `.cluster` part of Talos machine configuration can now be [applied immediately](../../guides/editing-machine-configuration) (without a reboot).
This allows, for example, updating versions of control plane components, adding additional arguments or modifying bootstrap manifests.
Future versions of Talos will expand on this to allow most of the machine configuration to be applied without a reboot.

## Disk Encryption

Talos now supports encryption for `STATE` and `EPHEMERAL` partitions of the system disk.
The `STATE` partition holds machine configuration and the `EPHEMERAL` partition is mounted as `/var` which stores container runtime
state, and configuration files laid on top of Talos read-only immutable root filesystem.
The encryption key in Talos 0.9 is derived from the Node UUID which is a unique machine identifier provided by the manufacturer.
Disk encryption is not enabled by default: it needs to be [enabled](../../guides/disk-encryption/) via machine configuration.

## Virtual IP for the Control Plane Endpoint

Talos adds support for Virtual L2 [shared IP](../../guides/vip/) for the control plane: control plane nodes ensure only one of the nodes
advertise the shared IP via ARP.
If one of the control plane nodes goes down, another node takes over the shared IP.

## Updated Components

Linux: 5.10.1 -> 5.10.19

Kubernetes: 1.20.1 -> 1.20.5

CoreDNS: 1.7.0 -> 1.8.0

etcd: 3.4.14 -> 3.4.15

containerd: 1.4.3 -> 1.4.4
