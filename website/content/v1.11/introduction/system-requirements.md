---
title: System Requirements
weight: 40
description: "Hardware requirements for running Talos Linux."
---

## Minimum Requirements

<table class="table-auto">
  <thead>
    <tr>
      <th class="px-4 py-2">Role</th>
      <th class="px-4 py-2">Memory</th>
      <th class="px-4 py-2">Cores</th>
      <th class="px-4 py-2">System Disk</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td class="border px-4 py-2">Control Plane</td>
      <td class="border px-4 py-2">2 GiB</td>
      <td class="border px-4 py-2">2</td>
      <td class="border px-4 py-2">10 GiB</td>
    </tr>
    <tr class="bg-gray-100">
      <td class="border px-4 py-2">Worker</td>
      <td class="border px-4 py-2">1 GiB</td>
      <td class="border px-4 py-2">1</td>
      <td class="border px-4 py-2">10 GiB</td>
    </tr>
  </tbody>
</table>

## Recommended

<table class="table-auto">
  <thead>
    <tr>
      <th class="px-4 py-2">Role</th>
      <th class="px-4 py-2">Memory</th>
      <th class="px-4 py-2">Cores</th>
      <th class="px-4 py-2">System Disk</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td class="border px-4 py-2">Control Plane</td>
      <td class="border px-4 py-2">4 GiB</td>
      <td class="border px-4 py-2">4</td>
      <td class="border px-4 py-2">100 GiB</td>
    </tr>
    <tr class="bg-gray-100">
      <td class="border px-4 py-2">Worker</td>
      <td class="border px-4 py-2">2 GiB</td>
      <td class="border px-4 py-2">2</td>
      <td class="border px-4 py-2">100 GiB</td>
    </tr>
  </tbody>
</table>

These requirements are similar to that of Kubernetes.

## Storage

Talos Linux itself only requires less than 100 MB of disk space, but the EPHEMERAL partition is used to store pulled images, container work directories, and so on. Thus a minimum of 10 GiB of disk space is required. 100 GiB is recommended.

Talos manages disk partitioning automatically during installation, creating EFI, META, STATE, and EPHEMERAL partitions. The EPHEMERAL partition then expands to fill all the space left after the first three. That space can either remain entirely with EPHEMERAL or be divided into additional user volumes, depending on your needs. See [Disk Layout]({{< relref "../talos-guides/configuration/disk-management.md" >}}) for details.

For production, it is often more efficient to dedicate a smaller disk for the Talos installation itself, and use additional disks for workload storage. Using a large, single disk for both system and workloads is supported, but may not be optimal depending on your environment.
