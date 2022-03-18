---
title: What's New in Talos 0.11
weight: 5
---

## Networking Configuration

Talos networking configuration was [completely rewritten](../../learn-more/networking-resources/) to be based on controllers
and resources.
There are no changes to the machine configuration, but any update to `.machine.network` can now
be applied in immediate mode (without a reboot).
Talos should be setting up network configuration much faster on boot now, not blocking on DHCP for unconfigured
interfaces and skipping the reset network step.

## Talos API RBAC

Limited [RBAC support](../../guides/rbac/) in Talos API is now enabled by default for Talos 0.11.
Default `talosconfig` has `os:admin` role embedded in the certificate so that all the APIs are available.
Certificates with reduced set of roles can be created with `talosctl config new` command.

When upgrading from Talos 0.10, RBAC is not enabled by default.
Before enabling RBAC, generate `talosconfig` with `os:admin` role first to make sure that administrator still has access to the cluster when RBAC is enabled.

List of available roles:

* `os:admin` role enables every Talos API
* `os:reader` role limits access to read-only APIs which do not return sensitive data
* `os:etcd:backup` role only allows `talosctl etcd snapshot` API call (for etcd backup automation)

## Default to Bootstrap workflow

The `init.yaml` is no longer an output of `talosctl gen config`.
We now encourage using the bootstrap API, instead of `init` node types, as we
intend on deprecating this machine type in the future.
The `init.yaml` and `controlplane.yaml` machine configs are identical with the
exception of the machine type.
Users can use a modified `controlplane.yaml` with the machine type set to
`init` if they would like to avoid using the bootstrap API.

## Component Updates

* containerd was updated to 1.5.2
* Linux kernel was updated to 5.10.45
* Kubernetes was updated to 1.21.2
* etcd was updated to 3.4.16

## CoreDNS

Added the flag `cluster.coreDNS.disabled` to coreDNS deployment during the cluster bootstrap.

## Legacy BIOS Support

Added an option to the `machine.install` section of the machine config that can enable marking MBR partition bootable
for the machines that have legacy BIOS which does not support GPT partitioning scheme.

## Multi-arch Installer

Talos installer image (for any arch) now contains artifacts for both `amd64` and `arm64` architecture.
This means that e.g. images for arm64 SBCs can be generated on amd64 host.
