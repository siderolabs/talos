---
title: "Network Connectivity"
weight: 80
description: "Description of the Networking Connectivity needed by Talos Linux"
aliases:
  - ../guides/configuring-network-connectivity
---

## Configuring Network Connectivity

The simplest way to deploy Talos is by ensuring that all the remote components of the system (`talosctl`, the control plane nodes, and worker nodes) all have layer 2 connectivity.
This is not always possible, however, so this page lays out the minimal network access that is required to configure and operate a talos cluster.

> Note: These are the ports required for Talos specifically, and should be configured _in addition_ to the ports required by kubernetes.
> See the [kubernetes docs](https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/install-kubeadm/#check-required-ports) for information on the ports used by kubernetes itself.

### Control plane node(s)

<table class="table-auto">
  <thead>
    <tr>
      <th class="px-4 py-2">Protocol</th>
      <th class="px-4 py-2">Direction</th>
      <th class="px-4 py-2">Port Range</th>
    <th class="px-4 py-2">Purpose</th>
    <th class="px-4 py-2">Used By</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td class="border px-4 py-2">TCP</td>
      <td class="border px-4 py-2">Inbound</td>
      <td class="border px-4 py-2">50000*</td>
    <td class="border px-4 py-2"><a href="../../learn-more/components/#apid">apid</a></td>
    <td class="border px-4 py-2">talosctl, control plane nodes</td>
    </tr>
    <tr>
      <td class="border px-4 py-2">TCP</td>
      <td class="border px-4 py-2">Inbound</td>
      <td class="border px-4 py-2">50001*</td>
    <td class="border px-4 py-2"><a href="../../learn-more/components/#trustd">trustd</a></td>
    <td class="border px-4 py-2">Worker nodes</td>
    </tr>
  </tbody>
</table>

> Ports marked with a `*` are not currently configurable, but that may change in the future.
> [Follow along here](https://github.com/siderolabs/talos/issues/1836).

### Worker node(s)

<table class="table-auto">
  <thead>
    <tr>
      <th class="px-4 py-2">Protocol</th>
      <th class="px-4 py-2">Direction</th>
      <th class="px-4 py-2">Port Range</th>
    <th class="px-4 py-2">Purpose</th>
    <th class="px-4 py-2">Used By</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td class="border px-4 py-2">TCP</td>
      <td class="border px-4 py-2">Inbound</td>
      <td class="border px-4 py-2">50000*</td>
    <td class="border px-4 py-2"><a href="../../learn-more/components/#apid">apid</a></td>
    <td class="border px-4 py-2">Control plane nodes</td>
    </tr>
  </tbody>
</table>

> Ports marked with a `*` are not currently configurable, but that may change in the future.
> [Follow along here](https://github.com/siderolabs/talos/issues/1836).
