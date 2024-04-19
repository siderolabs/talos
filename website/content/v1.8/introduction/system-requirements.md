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

Talos Linux itself only requires less than 100 MB of disk space, but the EPHEMERAL partition is used to store pulled images, container work directories, and so on.
Thus a minimum is 10 GiB of disk space is required.
100 GiB is desired.
Note, however, that because Talos Linux assumes complete control of the disk it is installed on, so that it can control the partition table for image based upgrades, you cannot partition the rest of the disk for use by workloads.

Thus it is recommended to install Talos Linux on a small, dedicated disk - using a Terabyte sized SSD for the Talos install disk would be wasteful.
Sidero Labs recommends having separate disks (apart from the Talos install disk) to be used for storage.
